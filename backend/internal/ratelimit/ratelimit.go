// Package ratelimit limita trabajo caro dentro de un proceso (TAL-201).
//
// Tiene dos piezas, ambas en memoria y solo con la biblioteca estándar:
//
//   - Failures cuenta intentos fallidos por clave (por ejemplo, el email de
//     un login) en una ventana fija y bloquea la clave al llegar al máximo.
//   - Slots acota cuántas operaciones caras corren a la vez (por ejemplo,
//     PBKDF2) y hace esperar, con tope, a las demás.
//
// La memoria basta porque cada dominio corre con una sola réplica
// (k8s/components/dominios). Con varias réplicas cada una contaría por su
// lado: el límite efectivo se multiplicaría por el número de réplicas.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// DefaultMaxKeys es cuántas claves guarda Failures como máximo. Evita que un
// atacante agote la memoria probando claves distintas: al llenarse, las
// claves nuevas no se cuentan (Slots sigue protegiendo la CPU).
const DefaultMaxKeys = 100_000

// Failures cuenta fallos por clave en una ventana fija. La ventana empieza
// con el primer fallo de la clave y dura window; al vencer, la cuenta vuelve
// a cero. Es seguro para uso concurrente.
type Failures struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	maxKeys   int
	now       func() time.Time
	entries   map[string]*failureEntry
	lastSweep time.Time
}

type failureEntry struct {
	count   int
	resetAt time.Time
}

// NewFailures crea un contador que bloquea una clave después de max fallos
// dentro de window. max y window deben ser positivos.
func NewFailures(max int, window time.Duration) *Failures {
	return &Failures{
		max:     max,
		window:  window,
		maxKeys: DefaultMaxKeys,
		now:     time.Now,
		entries: make(map[string]*failureEntry),
	}
}

// Blocked indica si key llegó al máximo de fallos en su ventana actual.
// Devuelve también cuánto falta para que la ventana venza (cero si no está
// bloqueada), para responder la cabecera Retry-After.
func (f *Failures) Blocked(key string) (bool, time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	entry, ok := f.entries[key]
	if !ok || !now.Before(entry.resetAt) {
		return false, 0
	}
	if entry.count < f.max {
		return false, 0
	}
	return true, entry.resetAt.Sub(now)
}

// Fail registra un fallo de key. Si la ventana de key ya venció, abre una
// nueva. Si el mapa está lleno aun después de barrer las vencidas, la clave
// nueva no se registra.
func (f *Failures) Fail(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	f.sweepLocked(now)
	entry, ok := f.entries[key]
	if ok && now.Before(entry.resetAt) {
		entry.count++
		return
	}
	if !ok && len(f.entries) >= f.maxKeys {
		return
	}
	f.entries[key] = &failureEntry{count: 1, resetAt: now.Add(f.window)}
}

// Reset borra los fallos de key; se llama tras un intento exitoso.
func (f *Failures) Reset(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, key)
}

// sweepLocked borra las claves vencidas como mucho una vez por ventana, o
// antes si el mapa se llenó. Debe llamarse con f.mu tomado.
func (f *Failures) sweepLocked(now time.Time) {
	if len(f.entries) < f.maxKeys && now.Sub(f.lastSweep) < f.window {
		return
	}
	for key, entry := range f.entries {
		if !now.Before(entry.resetAt) {
			delete(f.entries, key)
		}
	}
	f.lastSweep = now
}

// Slots acota cuántas operaciones corren a la vez. Es seguro para uso
// concurrente.
type Slots struct {
	slots chan struct{}
	wait  time.Duration
}

// NewSlots crea un límite de n operaciones simultáneas; quien llega con
// todos los cupos ocupados espera como máximo wait.
func NewSlots(n int, wait time.Duration) *Slots {
	return &Slots{slots: make(chan struct{}, n), wait: wait}
}

// Acquire toma un cupo. Devuelve false si no se liberó ninguno dentro de la
// espera máxima o si ctx terminó antes (por ejemplo, el cliente cortó). Cada
// Acquire que devuelve true debe tener su Release.
func (s *Slots) Acquire(ctx context.Context) bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
	}
	timer := time.NewTimer(s.wait)
	defer timer.Stop()
	select {
	case s.slots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Release devuelve el cupo tomado con Acquire.
func (s *Slots) Release() {
	<-s.slots
}
