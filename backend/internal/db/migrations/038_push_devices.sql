-- Напоминания без аккаунта: устройство подписывается со страницы недели и получает напоминания по своим
-- неделям. У такой подписки нет user_id; недели устройства — в push_plans, настройки — в самой подписке,
-- журнал отправленного — в push_sent (у подписок аккаунта журнал прежний, notifications_sent).
ALTER TABLE push_subscriptions ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE push_subscriptions ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Недели устройства. created_at освежается при каждом открытии недели: устройство, которое давно
-- не открывало недель, замолкает само (см. Push.Devices).
CREATE TABLE IF NOT EXISTS push_plans (
    endpoint   TEXT NOT NULL REFERENCES push_subscriptions(endpoint) ON DELETE CASCADE,
    plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (endpoint, plan_id)
);
CREATE INDEX IF NOT EXISTS push_plans_plan ON push_plans (plan_id);

CREATE TABLE IF NOT EXISTS push_sent (
    endpoint TEXT NOT NULL REFERENCES push_subscriptions(endpoint) ON DELETE CASCADE,
    key      TEXT NOT NULL,
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (endpoint, key)
);
