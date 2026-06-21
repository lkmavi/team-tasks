package mysql

import (
	"errors"
	"fmt"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

func uuidToBytes(id uuid.UUID) []byte { return id[:] }

func bytesToUUID(b []byte) (uuid.UUID, error) {
	if len(b) != 16 {
		return uuid.UUID{}, fmt.Errorf("expected 16 bytes, got %d", len(b))
	}
	var id uuid.UUID
	copy(id[:], b)
	return id, nil
}

func bytesToUUIDPtr(b []byte) (*uuid.UUID, error) {
	if len(b) == 0 {
		return nil, nil
	}
	id, err := bytesToUUID(b)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func isDuplicateEntry(err error) bool {
	var me *mysqldrv.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
