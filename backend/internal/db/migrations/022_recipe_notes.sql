-- Заметки к рецепту по языкам: почему так готовим, чем заменить, где ошибаются, как хранить, с чем подать.
-- Заливаются из seed (recipes_notes.json), показываются на странице рецепта разделом «Советы».
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS notes JSONB NOT NULL DEFAULT '{}';
