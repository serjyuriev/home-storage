CREATE DOMAIN non_negative_numeric AS NUMERIC CHECK (VALUE >= 0);

-- скорость использования вещей
CREATE TYPE consumption_rate AS (
    amount  NUMERIC,
    period  INTERVAL
);

-- категории вещей
CREATE TABLE categories (
    id          SERIAL PRIMARY KEY,
    name        TEXT   NOT NULL UNIQUE,
    icon_emoji  TEXT
);

CREATE TABLE items (
    id                    SERIAL               PRIMARY KEY,
    name                  TEXT                 NOT NULL,
    category_id           INT                  REFERENCES categories(id) ON DELETE SET NULL,
    qty_current           non_negative_numeric NOT NULL, -- текущее количество вещей
    qty_restock_threshold non_negative_numeric NOT NULL, -- уставка для определения необходимости дозакупки
    rate                  consumption_rate,
    unit                  TEXT                 NOT NULL DEFAULT 'pcs',
    notes                 TEXT,
    updated_at            TIMESTAMPTZ          NOT NULL
);

-- история изменения количества вещей (с партиционированием)
CREATE TABLE stock_history (
    id             BIGSERIAL,
    item_id        INT         NOT NULL,
    change_amount  NUMERIC     NOT NULL, -- положительное - закупка; отрицательное - использование
    qty_after      NUMERIC     NOT NULL, -- количество после изменения
    reason         TEXT,
    changed_at     TIMESTAMPTZ NOT NULL
) PARTITION BY RANGE (changed_at);

CREATE TABLE stock_history_2024
    PARTITION OF stock_history
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');

CREATE TABLE stock_history_2025
    PARTITION OF stock_history
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');

CREATE TABLE stock_history_2026
    PARTITION OF stock_history
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE TABLE stock_history_default
    PARTITION OF stock_history
    DEFAULT;

CREATE INDEX idx_stock_history_item_id ON stock_history (item_id);
CREATE INDEX idx_stock_history_changed_at ON stock_history (changed_at);

-- получить список вещей, которые заканчиваются (кол-во <= уставки)
CREATE OR REPLACE FUNCTION get_running_low()
RETURNS SETOF items
LANGUAGE sql
STABLE
AS $$
    SELECT *
    FROM   items
    WHERE  qty_current <= qty_restock_threshold
    ORDER  BY (qty_current / qty_restock_threshold) ASC;
$$;

-- функция для расчета ориентировочного количества дней, через которое вещь закончится, основываясь на consumption_rate
CREATE OR REPLACE FUNCTION estimate_days_remaining(p_item_id INT)
RETURNS NUMERIC
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_item          items%ROWTYPE;
    v_daily_usage   NUMERIC;
    v_days          NUMERIC;
BEGIN
    SELECT * INTO v_item FROM items WHERE id = p_item_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Item with id % does not exist', p_item_id;
    END IF;

    IF v_item.rate IS NULL OR (v_item.rate).amount IS NULL OR (v_item.rate).period IS NULL THEN
        RETURN NULL;
    END IF;

    v_daily_usage := (v_item.rate).amount / NULLIF(EXTRACT(EPOCH FROM (v_item.rate).period) / 86400, 0);

    IF v_daily_usage = 0 THEN
        RETURN NULL;
    END IF;

    v_days := v_item.qty_current / v_daily_usage;

    RETURN ROUND(v_days, 1);
END;
$$;

-- функция для записи в табличку истории (через триггер)
CREATE OR REPLACE FUNCTION record_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.qty_current IS DISTINCT FROM OLD.qty_current THEN
        INSERT INTO stock_history (item_id, change_amount, qty_after, reason)
        VALUES (
            NEW.id,
            NEW.qty_current - OLD.qty_current,
            NEW.qty_current,
            TG_ARGV[0]
        );
    END IF;

    RETURN NEW;
END;
$$;

-- хранимая процедура для реализации процесса "потребления" вещи
CREATE OR REPLACE PROCEDURE log_usage(
    p_item_id     INT,
    p_amount_used NUMERIC,
    p_reason      TEXT DEFAULT 'used'
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_current NUMERIC;
BEGIN
    IF p_amount_used <= 0 THEN
        RAISE EXCEPTION 'amount_used must be greater than zero, got %', p_amount_used;
    END IF;

    SELECT qty_current INTO v_current FROM items WHERE id = p_item_id FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Item with id % does not exist', p_item_id;
    END IF;

    IF v_current - p_amount_used < 0 THEN
        RAISE EXCEPTION 'Not enough item in stock. Have %, trying to use %', v_current, p_amount_used;
    END IF;

    UPDATE items SET qty_current = qty_current - p_amount_used WHERE id = p_item_id;

    COMMIT;
END;
$$;

-- хранимая процедура для реализации процесса "пополнения" вещи
CREATE OR REPLACE PROCEDURE restock_item(
    p_item_id    INT,
    p_amount     NUMERIC,
    p_price_paid NUMERIC DEFAULT NULL
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_reason TEXT;
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'Restock amount must be greater than zero, got %', p_amount;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM items WHERE id = p_item_id) THEN
        RAISE EXCEPTION 'Item with id % does not exist', p_item_id;
    END IF;

    v_reason := CASE
        WHEN p_price_paid IS NOT NULL THEN FORMAT('restock (paid: %s)', p_price_paid)
        ELSE 'restock'
    END;

    UPDATE items SET qty_current = qty_current + p_amount WHERE id = p_item_id;

    INSERT INTO stock_history (item_id, change_amount, qty_after, reason)
    SELECT p_item_id, p_amount, qty_current, v_reason
    FROM items
    WHERE id = p_item_id;

    COMMIT;
END;
$$;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON items
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_items_history
    AFTER UPDATE ON items
    FOR EACH ROW
    EXECUTE FUNCTION record_change();

-- дашборд
CREATE OR REPLACE VIEW v_stock_status AS
SELECT
    i.id,
    i.name,
    c.name AS category,
    c.icon_emoji,
    i.qty_current,
    i.qty_restock_threshold,
    i.unit,
    estimate_days_remaining(i.id) AS days_remaining,
    CASE
        WHEN i.qty_current = 0                        THEN 'out'
        WHEN i.qty_current <= i.qty_restock_threshold THEN 'low'
        ELSE                                               'ok'
    END AS status,
    i.notes,
    i.updated_at
FROM items AS i
LEFT JOIN categories AS c ON c.id = i.category_id
ORDER BY
    CASE
        WHEN i.qty_current = 0                        THEN 0
        WHEN i.qty_current <= i.qty_restock_threshold THEN 1
        ELSE                                               2
    END,
    i.name;

-- дашборд использования вещей помесячно
CREATE MATERIALIZED VIEW mv_monthly_consumption AS
SELECT
    i.id AS item_id,
    i.name AS item_name,
    c.name AS category,
    DATE_TRUNC('month', sh.changed_at)::DATE AS month,
    COALESCE(SUM(sh.change_amount) FILTER (WHERE sh.change_amount < 0) * -1, 0) AS total_consumed,
    COALESCE(SUM(sh.change_amount) FILTER (WHERE sh.change_amount > 0), 0) AS total_restocked,
    COALESCE(SUM(sh.change_amount), 0) AS net_change
FROM items AS i
LEFT JOIN categories AS c ON c.id = i.category_id
LEFT JOIN stock_history AS sh ON sh.item_id = i.id
GROUP BY i.id, i.name, c.name, DATE_TRUNC('month', sh.changed_at)
ORDER BY month DESC, i.name;

CREATE UNIQUE INDEX idx_mv_monthly_consumption_pk ON mv_monthly_consumption (item_id, month);
CREATE INDEX idx_mv_monthly_consumption_month ON mv_monthly_consumption (month);

-- базовые категории
INSERT INTO categories (name, icon_emoji) VALUES
    ('Bathroom',  '🛁'),
    ('Pantry',    '🍚'),
    ('Cleaning',  '🧹'),
    ('Laundry',   '🧺');