// +build random

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/google/uuid"
)

func idInitImpl() {
	guid := uuid.New()
	h := sha256.New()

	h.Write([]byte(guid.String()))
	cipher := h.Sum(nil)

	str := fmt.Sprintf("%s", hex.EncodeToString(cipher))
	ID = str[:12]

	logrus.Infof("随机生成ID: %s\n", ID)
}
