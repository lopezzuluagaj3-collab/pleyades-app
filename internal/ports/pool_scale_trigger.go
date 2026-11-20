package ports

import "context"

// PoolScaleTrigger define el disparador para solicitar un nuevo servidor ADBFP cuando se alcanza el umbral de capacidad
type PoolScaleTrigger interface {
	TriggerWarmPool(ctx context.Context, serverName string) error
}
