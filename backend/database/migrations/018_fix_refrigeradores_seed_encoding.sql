-- TAL-231: repara dos nombres del seed de refrigeradores que quedaron con la
-- tilde dañada.
--
-- En las bases sembradas desde Windows (el clúster UCT entre ellas) el seed de
-- 003 se cargó con la codificación del cliente equivocada y cada "é" quedó
-- guardada como dos caracteres. 019_reconcile_refrigeradores_category.sql
-- busca la categoría "Electrodomésticos" por su nombre y compara la
-- especificación "Eficiencia energética" con la de Refrigeracion: con los
-- nombres dañados aborta. Por eso esta migración va antes en
-- migration_order.txt.
--
-- Alcance, a propósito estrecho: solo los dos nombres que necesita 019. Los
-- demás textos dañados del seed (regiones, descripciones de servicios) no se
-- tocan aquí.
--
-- Los nombres van con escapes Unicode para no depender de la codificación con
-- que se aplique el archivo:
--   \00E9       = e con tilde (texto correcto)
--   \251C\00AE  = los dos caracteres con que quedó guardada
--
-- Todo va en un solo bloque: o se aplica completo o no cambia nada. Es
-- idempotente: en una base con los nombres correctos no hace nada.
DO $$
BEGIN
    UPDATE product_category
    SET name = U&'Electrodom\00E9sticos'
    WHERE parent_category_id IS NULL
      AND name = U&'Electrodom\251C\00AEsticos'
      AND NOT EXISTS (
          SELECT 1 FROM product_category ok
          WHERE ok.parent_category_id IS NULL
            AND ok.name = U&'Electrodom\00E9sticos');

    UPDATE product_category_specification s
    SET name = U&'Eficiencia energ\00E9tica'
    WHERE s.name = U&'Eficiencia energ\251C\00AEtica'
      AND NOT EXISTS (
          SELECT 1 FROM product_category_specification ok
          WHERE ok.category_id = s.category_id
            AND ok.name = U&'Eficiencia energ\00E9tica');
END;
$$;
