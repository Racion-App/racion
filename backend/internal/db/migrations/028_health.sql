-- Замеры для страницы состояния /status: раз в минуту по компонентам, хранятся месяц.
CREATE TABLE IF NOT EXISTS health_samples (
    ts        TIMESTAMPTZ NOT NULL DEFAULT now(),
    component TEXT NOT NULL,
    ok        BOOLEAN NOT NULL,
    ms        INT NOT NULL
);
CREATE INDEX IF NOT EXISTS health_samples_component_ts ON health_samples (component, ts DESC);
