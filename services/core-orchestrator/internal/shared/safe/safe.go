// Package safe corre trabajo de fondo bajo una barrera de panic: un panic en
// una goroutine de consumer, worker o scheduler se loguea (con stack) y se
// contiene, en vez de tumbar todo el proceso — la API incluida.
//
// Regla de uso:
//   - Do        → alrededor de UNA unidad de trabajo (una fila de cola, una
//     conexión, un mensaje). El panic aborta ese ítem; el loop sigue.
//   - Supervise → alrededor de un loop bloqueante de fondo (un Start). Si el
//     loop panic-ea, se reinicia tras una pausa hasta que ctx se cancele.
//   - Guard     → cuando quien llama necesita REACCIONAR al panic (p. ej. un
//     consumer decidiendo cómo hacer ack) en vez de solo tragárselo.
//
// No usar en el wiring de arranque de cmd/: un repo nil o un TLS mal
// configurado deben reventar fuerte en el deploy.
package safe

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

// superviseRestartDelay es la pausa antes de reiniciar un loop supervisado que
// panic-eó, para que un panic en bucle cerrado no sature CPU ni el log.
const superviseRestartDelay = 2 * time.Second

// Do ejecuta fn recuperando cualquier panic y logueándolo con stack trace.
// Devuelve true si fn terminó sin panic-ear.
func Do(name string, fn func()) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic recuperado en %s: %v\n%s", name, r, debug.Stack())
			ok = false
		}
	}()
	fn()
	return true
}

// Supervise ejecuta fn (un loop de fondo bloqueante, p. ej. Worker.Start) y,
// si panic-ea, loguea el stack y lo reinicia tras superviseRestartDelay —
// repitiendo hasta que ctx se cancele. Un retorno limpio de fn (ctx cancelado)
// termina la supervisión. Los Start de este servicio solo retornan ante
// ctx.Done, así que un retorno con ctx todavía vivo se trata como panic
// recuperado y se reinicia igual. Llamar con `go safe.Supervise(ctx, ...)`.
func Supervise(ctx context.Context, name string, fn func(context.Context)) {
	for {
		if ctx.Err() != nil {
			return
		}

		Do(name, func() { fn(ctx) })

		if ctx.Err() != nil {
			return
		}

		log.Printf("%s: el loop terminó antes de tiempo, reiniciando en %s", name, superviseRestartDelay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(superviseRestartDelay):
		}
	}
}

// PanicError envuelve el valor y el stack de un panic recuperado por Guard,
// para que quien llama lo distinga de un error normal con errors.As.
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v\n%s", e.Value, e.Stack)
}

// Guard ejecuta fn y devuelve su error, convirtiendo un panic en un
// *PanicError en vez de dejarlo propagar. Para cuando quien llama necesita
// actuar según haya sido panic o error normal.
func Guard(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r, Stack: debug.Stack()}
		}
	}()
	return fn()
}
