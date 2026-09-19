-- Редакционные подборки заливаются из seed (collections.json) при старте, владельца-пользователя у них нет.
ALTER TABLE collections ALTER COLUMN user_id DROP NOT NULL;
