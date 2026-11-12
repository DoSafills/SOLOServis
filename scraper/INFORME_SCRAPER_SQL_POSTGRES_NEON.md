# Informe: exportación SQL del scraper para PostgreSQL y Neon

Fecha: 2026-11-04

## Objetivo

Generar, al finalizar una ejecución del scraper que haya extraído productos,
un archivo SQL revisable para importar manualmente los datos en PostgreSQL
local o Neon. El scraper no se conecta ni selecciona una base de datos.

## Cambios realizados

- Se agregó `src/scraper/output/sql.py`, que escribe
  `out/sql/run-<id>-<tienda>.sql`.
- El SQL registra o reutiliza marcas y productos, inserta imágenes faltantes
  y crea o actualiza las ofertas con precios, disponibilidad, despacho y URL.
- El script usa las tablas y columnas del esquema PostgreSQL del proyecto,
  verifica que la tienda esté activa y resuelve la categoría mediante
  `categories.legacy_product_category_id`.
- La salida de `scraper run` y `out/runs/<id>/run.json` incluyen la ruta
  `sqlFile` y cualquier error de generación en `sqlError`.
- Se documentó la importación manual y se añadieron pruebas del generador y
  su integración con el pipeline.

## Uso con la base seleccionada para el backend

Desde la raíz del repositorio, elegir la base activa y aplicar el archivo
generado correspondiente:

```powershell
.\scripts\switch-database.ps1 -Database postgres # o neon
$dbUrl = (Get-Content .env | Where-Object { $_ -match '^DATABASE_URL=' } |
    Select-Object -First 1) -replace '^DATABASE_URL=', ''
psql "$dbUrl" -v ON_ERROR_STOP=1 -f .\scraper\out\sql\run-<id>-ripley-cl.sql
```

La tienda activa y la categoría deben existir en la base seleccionada. Revisar
el SQL antes de ejecutarlo; el archivo se genera para importación manual y no
reemplaza ni modifica `.env`.

## Validación

- Pruebas dirigidas: `tests/test_sql_output.py` y `tests/test_pipeline.py`;
  resultado: 8 pruebas pasaron.
- Ruff sobre los archivos modificados: sin errores.
- `git diff --check`: sin errores.
- No se ejecutó el SQL contra PostgreSQL o Neon; la importación real debe
  validarse primero en la base elegida por el usuario.
