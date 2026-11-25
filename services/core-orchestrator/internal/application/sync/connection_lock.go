package sync

import (
	"context"
	"sync"
)

// connectionLocks serializa, por conexión, todo lo que reescribe sus ajustes de
// autenticación: renovar el token, vincular, editar la aplicación y restaurar.
// Una renovación guarda varios ajustes por separado; sin esto, un respaldo
// podría copiar una mezcla de valores viejos y nuevos.
//
// Vale dentro de un proceso: core-orchestrator corre con una sola réplica.
type connectionLocks struct {
	mu    sync.Mutex
	locks map[int64]chan struct{}
}

// acquire espera el turno de la conexión y devuelve cómo liberarlo. No es
// reentrante.
func (c *connectionLocks) acquire(ctx context.Context, connectionID int64) (func(), error) {
	c.mu.Lock()
	if c.locks == nil {
		c.locks = make(map[int64]chan struct{})
	}
	lock, ok := c.locks[connectionID]
	if !ok {
		lock = make(chan struct{}, 1)
		c.locks[connectionID] = lock
	}
	c.mu.Unlock()

	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
