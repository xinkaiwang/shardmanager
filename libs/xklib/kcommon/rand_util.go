package kcommon

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

const defaultCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type SafeRand struct {
	mu         sync.Mutex
	seededRand *rand.Rand
}

var safeRand SafeRand

type OpGetRand func(*rand.Rand)

func GetRandom(ctx context.Context, op OpGetRand) {
	safeRand.mu.Lock()
	defer func() {
		safeRand.mu.Unlock()
	}()
	if safeRand.seededRand == nil {
		buf := make([]byte, 8)
		_, err := crypto_rand.Read(buf)
		if err != nil {
			klogging.Warning(ctx).WithError(err).Log("CryptoRandSeedFailed", "")
		} else {
			seed := int64(binary.BigEndian.Uint64(buf))
			safeRand.seededRand = rand.New(rand.NewSource(seed))
			klogging.Info(ctx).With("seed", strconv.FormatInt(seed, 16)).Log("CryptoRandSeedSucc", "")
		}
	}
	op(safeRand.seededRand)
}

func StringWithCharset(ctx context.Context, length int, charset string) string {
	b := make([]byte, length)
	GetRandom(ctx, func(r *rand.Rand) {
		for i := range b {
			b[i] = charset[r.Intn(len(charset))]
		}
	})
	return string(b)
}

func RandomString(ctx context.Context, length int) string {
	return StringWithCharset(ctx, length, defaultCharset)
}

// pseudo-random number in [0,n)
func RandomInt(ctx context.Context, max int) (ret int) {
	GetRandom(ctx, func(r *rand.Rand) {
		ret = r.Intn(max)
	})
	return
}

// pseudo-random number in [0,n)
func RandomUint64(ctx context.Context, max uint64) (ret uint64) {
	GetRandom(ctx, func(r *rand.Rand) {
		ret = r.Uint64() % max
	})
	return
}

func RoundDurationToMs(ctx context.Context, duration time.Duration) int64 {
	return (duration.Microseconds() + int64(RandomInt(ctx, 1000))) / 1000
}

// For example, val=100 ratio=0.1 means return a random value between [90-110)
func RandomlizeValueByRatio(ctx context.Context, value int, ratio float32) int {
	min := int(float32(value) * (1. - ratio))
	max := int(float32(value) * (1. + ratio))
	return RandomInt(ctx, max-min) + min
}

func NewTraceId(ctx context.Context, prefix string, size int) string {
	return prefix + RandomString(ctx, size)
}
