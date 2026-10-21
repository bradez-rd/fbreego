package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"sync/atomic"
	"time"
)

var requestCounter atomic.Uint64

func init() {
	requestCounter.Store(1)
}

// NextRequestBase36 incrementa atómicamente el contador de solicitudes y devuelve su representación en Base36.
func NextRequestBase36() string {
	val := requestCounter.Add(1) - 1
	return strconv.FormatUint(val, 36)
}

// ToBase36 convierte un entero sin signo a cadena en Base36 minúscula.
func ToBase36(n uint64) string {
	return strconv.FormatUint(n, 36)
}

// RandStr genera una cadena aleatoria de longitud fija compuesta por caracteres alfanuméricos minúsculos.
func RandStr(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateClientID genera un identificador UUID-like con formato 8-4-4-4-12.
func GenerateClientID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		RandStr(8),
		RandStr(4),
		RandStr(4),
		RandStr(4),
		RandStr(12),
	)
}

// GenerateSessionID genera un entero pseudoaleatorio entre 1 y 2^53 (compatible con JavaScript MAX_SAFE_INTEGER).
func GenerateSessionID() int64 {
	max := new(big.Int).Lsh(big.NewInt(1), 53)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return time.Now().UnixNano()
	}
	return n.Int64() + 1
}

// GenThreadingID genera el identificador numérico de 64 bits para mensajes de Messenger (otid / threading_id).
func GenThreadingID() string {
	nowMillis := uint64(time.Now().UnixMilli())
	rndVal, _ := rand.Int(rand.Reader, big.NewInt(4294967295))
	rndBits := rndVal.Uint64() & 0x3FFFFF // Últimos 22 bits (2^22 - 1)

	// Desplazar timestamp a la izquierda 22 bits y sumar los 22 bits aleatorios
	combined := (nowMillis << 22) | rndBits
	return strconv.FormatUint(combined, 10)
}
