package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock es un reloj manual para mover el tiempo en los tests.
type fakeClock struct{ t time.Time }

// now devuelve la hora actual del reloj falso.
func (c *fakeClock) now() time.Time { return c.t }

// newTestFailures crea un Failures con reloj falso.
func newTestFailures(max int, window time.Duration) (*Failures, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	f := NewFailures(max, window)
	f.now = clock.now
	return f, clock
}

// TestFailures fija cuándo una clave queda bloqueada, el Retry-After y el
// desbloqueo por vencimiento o Reset.
func TestFailures(t *testing.T) {
	cases := []struct {
		name        string
		fails       int
		advance     time.Duration
		reset       bool
		wantBlocked bool
		wantRetry   time.Duration
	}{
		{name: "sin fallos", fails: 0, wantBlocked: false},
		{name: "bajo el máximo", fails: 2, wantBlocked: false},
		{name: "en el máximo bloquea", fails: 3, wantBlocked: true, wantRetry: time.Minute},
		{name: "retry-after descuenta lo transcurrido", fails: 3, advance: 20 * time.Second,
			wantBlocked: true, wantRetry: 40 * time.Second},
		{name: "la ventana vence", fails: 3, advance: time.Minute, wantBlocked: false},
		{name: "reset desbloquea", fails: 5, reset: true, wantBlocked: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, clock := newTestFailures(3, time.Minute)
			for i := 0; i < tc.fails; i++ {
				f.Fail("ana@mail.cl")
			}
			if tc.reset {
				f.Reset("ana@mail.cl")
			}
			clock.t = clock.t.Add(tc.advance)
			blocked, retry := f.Blocked("ana@mail.cl")
			if blocked != tc.wantBlocked || retry != tc.wantRetry {
				t.Fatalf("Blocked = %v, %v; quiero %v, %v", blocked, retry, tc.wantBlocked, tc.wantRetry)
			}
			if other, _ := f.Blocked("otra@mail.cl"); other {
				t.Fatal("una clave bloqueada no debe afectar a otra")
			}
		})
	}
}

// TestFailuresNuevaVentanaTrasVencer: un fallo después de vencer la ventana
// abre una nueva con la cuenta en uno.
func TestFailuresNuevaVentanaTrasVencer(t *testing.T) {
	f, clock := newTestFailures(2, time.Minute)
	f.Fail("k")
	f.Fail("k")
	clock.t = clock.t.Add(time.Minute)
	f.Fail("k")
	if blocked, _ := f.Blocked("k"); blocked {
		t.Fatal("un fallo en la ventana nueva no debe bloquear: la cuenta debía volver a cero")
	}
}

// TestFailuresTopeDeClaves: con el mapa lleno las claves nuevas no se
// cuentan, y el barrido de vencidas vuelve a hacer espacio.
func TestFailuresTopeDeClaves(t *testing.T) {
	f, clock := newTestFailures(1, time.Minute)
	f.maxKeys = 3
	for i := 0; i < 5; i++ {
		f.Fail(fmt.Sprintf("k%d", i))
	}
	if len(f.entries) != 3 {
		t.Fatalf("entries = %d, quiero 3 (tope)", len(f.entries))
	}
	if blocked, _ := f.Blocked("k4"); blocked {
		t.Fatal("una clave fuera del tope no se cuenta")
	}
	// Al vencer la ventana, el barrido libera espacio para claves nuevas.
	clock.t = clock.t.Add(time.Minute)
	f.Fail("nueva")
	if blocked, _ := f.Blocked("nueva"); !blocked {
		t.Fatal("tras barrer las vencidas, la clave nueva debe contarse")
	}
	if len(f.entries) != 1 {
		t.Fatalf("entries = %d, quiero 1 tras el barrido", len(f.entries))
	}
}

// TestFailuresConcurrente: Fail y Blocked desde muchas goroutines no
// pierden cuentas (correr con -race).
func TestFailuresConcurrente(t *testing.T) {
	f := NewFailures(1000, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				f.Fail("k")
				f.Blocked("k")
			}
		}()
	}
	wg.Wait()
	if blocked, _ := f.Blocked("k"); !blocked {
		t.Fatal("1000 fallos concurrentes deben llegar al máximo")
	}
}

// TestSlots fija la toma y liberación de cupos, la espera máxima y la
// cancelación por contexto.
func TestSlots(t *testing.T) {
	t.Run("toma y libera", func(t *testing.T) {
		s := NewSlots(1, 10*time.Millisecond)
		if !s.Acquire(context.Background()) {
			t.Fatal("el primer cupo debe estar libre")
		}
		s.Release()
		if !s.Acquire(context.Background()) {
			t.Fatal("tras Release el cupo vuelve a estar libre")
		}
	})
	t.Run("lleno vence la espera", func(t *testing.T) {
		s := NewSlots(1, 10*time.Millisecond)
		s.Acquire(context.Background())
		start := time.Now()
		if s.Acquire(context.Background()) {
			t.Fatal("con el cupo ocupado Acquire debe fallar")
		}
		if waited := time.Since(start); waited < 10*time.Millisecond {
			t.Fatalf("esperó %v, quiero al menos la espera máxima", waited)
		}
	})
	t.Run("espera y obtiene el cupo liberado", func(t *testing.T) {
		s := NewSlots(1, time.Second)
		s.Acquire(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			s.Release()
		}()
		if !s.Acquire(context.Background()) {
			t.Fatal("debía obtener el cupo liberado durante la espera")
		}
	})
	t.Run("contexto cancelado", func(t *testing.T) {
		s := NewSlots(1, time.Second)
		s.Acquire(context.Background())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if s.Acquire(ctx) {
			t.Fatal("con el contexto cancelado Acquire debe fallar")
		}
	})
}
