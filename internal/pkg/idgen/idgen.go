package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func New(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_%s%s", prefix, time.Now().UTC().Format("20060102150405"), hex.EncodeToString(b[:]))
}

func ThreadID() string { return New("thr") }
func TurnID() string   { return New("trn") }
func ItemID() string   { return New("itm") }
