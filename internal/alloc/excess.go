package alloc

import "task126-reinsurance/internal/domain"

// ExcessLayer computes the recovery for a single loss (or accumulated event)
// under an excess contract, given the available limit remaining.
//
//   attachmentConsumed = clamp(loss, 0, attachment)        -- absorbed by retention
//   exposed            = loss - attachmentConsumed          -- above retention
//   layerRecovery      = clamp(exposed, 0, limitRemaining) -- absorbed by contract
//   retentionAbove     = exposed - layerRecovery           -- above limit, retained
//
// The contract pays layerRecovery; the cedant retains attachmentConsumed +
// retentionAbove. limitConsumed == layerRecovery is what gets decremented from
// the remaining limit.
func ExcessLayer(loss, attachment, limitRemaining domain.Money) (attachmentConsumed, layerRecovery, retentionAbove domain.Money) {
	attachmentConsumed = loss.Min(attachment).Max(0)
	exposed := loss.Sub(attachmentConsumed).Max(0)
	layerRecovery = exposed.Min(limitRemaining).Max(0)
	retentionAbove = exposed.Sub(layerRecovery).Max(0)
	return
}
