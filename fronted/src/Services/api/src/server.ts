import express from 'express'
import cors from 'cors'
import 'dotenv/config'
import productsRouter from './routes/products.js'
import servicesRouter from './routes/services.js'
import storesRouter from './routes/stores.js'

const app = express()

app.use(cors())
app.use(express.json())

app.get('/api/health', (_req, res) => res.json({ status: 'ok' }))

app.use('/api/products', productsRouter)
app.use('/api/services', servicesRouter)
app.use('/api/stores', storesRouter)

const PORT = process.env.PORT || 4000
app.listen(PORT, () => {
  console.log(`API escuchando en http://localhost:${PORT}`)
})
