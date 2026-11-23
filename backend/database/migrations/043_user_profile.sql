-- 043: foto de perfil del usuario. Se guarda en la misma fila de user_account
-- para no depender de un volumen escribible (los pods usan
-- readOnlyRootFilesystem) y para que sobreviva a los reinicios del pod.
-- avatar_data y avatar_mime van juntos: o hay foto con su tipo, o no hay foto.
ALTER TABLE user_account
    ADD COLUMN avatar_data BYTEA,
    ADD COLUMN avatar_mime TEXT;

ALTER TABLE user_account
    ADD CONSTRAINT user_account_avatar_pair
        CHECK ((avatar_data IS NULL) = (avatar_mime IS NULL)),
    ADD CONSTRAINT user_account_avatar_mime
        CHECK (avatar_mime IS NULL OR avatar_mime IN ('image/jpeg', 'image/png', 'image/webp'));
