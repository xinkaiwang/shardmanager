package kcommon

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

const defaultCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// cryptoReader is the entropy source for seeding; a var so tests can inject
// a failing reader (KLOG-013).
var cryptoReader io.Reader = crypto_rand.Reader

// SafeRand: crypto-seeded math/rand PRNG behind a mutex.
//
// CONTRACT (KLOG-013①): the output stream is NOT cryptographically secure —
// after the 64-bit seed everything is deterministic. Fine for IDs (needs
// uniqueness) and jitter (needs statistical spread); NEVER use RandomXxx for
// tokens, session ids, or anything an attacker must not predict — use
// crypto/rand directly for those.
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
		_, err := io.ReadFull(cryptoReader, buf)
		if err != nil {
			// KLOG-013③ fail-fast: the old path logged a Warn, left
			// seededRand nil, and nil-deref'd inside op — "wrote a
			// fallback, shipped a crash". A dead entropy source at first
			// use is process-level infrastructure failure: die loudly with
			// a typed error at the real cause.
			ke := kerror.Create("CryptoRandSeedFailed", "cannot seed PRNG from crypto/rand").
				With("error", err.Error())
			panic(ke)
		}
		seed := int64(binary.BigEndian.Uint64(buf))
		safeRand.seededRand = rand.New(rand.NewSource(seed))
		// KLOG-013② hygiene: do not log the seed value — with a
		// deterministic PRNG it would let any log reader replay the whole
		// stream. The event (without the seed) is Debug-grade at best.
		slog.DebugContext(ctx, "PRNG seeded from crypto/rand",
			slog.String("event", "CryptoRandSeeded"))
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
