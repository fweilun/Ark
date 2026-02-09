// README: Location snapshot for persistence and replay.
package location

import (
    "time"

    "ark/internal/types"
)

type Snapshot struct {
    ID         int64
    UserID     types.ID
    UserType   string
    Position   types.Point
    RecordedAt time.Time
}
