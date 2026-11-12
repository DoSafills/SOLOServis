-- Backup previo de filas potencialmente afectadas por el índice único normalizado.
CREATE TABLE IF NOT EXISTS public.backup_019_email_case_insensitive AS
SELECT ua.*, now() AS backed_up_at
FROM public.user_account ua
WHERE lower(btrim(ua.email)) IN (
    SELECT lower(btrim(email))
    FROM public.user_account
    GROUP BY 1
    HAVING count(*) > 1
);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_user_account_email_norm
    ON public.user_account (lower(btrim(email)));

-- Rollback:
-- DROP INDEX CONCURRENTLY IF EXISTS public.uq_user_account_email_norm;
