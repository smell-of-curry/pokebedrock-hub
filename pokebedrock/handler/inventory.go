// Package handler provides handlers for the server.
package handler

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
)

// InventoryHandler ...
type InventoryHandler struct {
	inventory.NopHandler
}

// HandleTake ...
func (InventoryHandler) HandleTake(ctx *inventory.Context, _ int, _ *item.Stack) {
	ctx.Cancel()
}

// HandlePlace ...
func (InventoryHandler) HandlePlace(ctx *inventory.Context, _ int, _ *item.Stack) {
	ctx.Cancel()
}

// HandleDrop ...
func (InventoryHandler) HandleDrop(ctx *inventory.Context, _ int, _ *item.Stack) {
	ctx.Cancel()
}
