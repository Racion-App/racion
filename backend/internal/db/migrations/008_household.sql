-- Семья: состав (взрослые и дети) и аккаунты, которые её делят. Планы всех аккаунтов семьи общие.
CREATE TABLE IF NOT EXISTS households (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL DEFAULT '',
    adults       JSONB NOT NULL DEFAULT '[]',   -- [{name, goal, appetite, slots}]
    kids         JSONB NOT NULL DEFAULT '[]',   -- [{name, ageMonths, feeding, formula, formulaBrand, formulaMl}]
    invite_token TEXT UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS household_users (
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,  -- аккаунт состоит в одной семье
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, user_id)
);
