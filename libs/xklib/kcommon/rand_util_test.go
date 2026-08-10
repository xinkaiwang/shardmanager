package kcommon

import (
	"context"
	"errors"
	"testing"
)

func TestRandomString_Basic(t *testing.T) {
	ctx := context.Background()
	s1 := RandomString(ctx, 16)
	s2 := RandomString(ctx, 16)
	if len(s1) != 16 || len(s2) != 16 {
		t.Fatalf("length: %q %q", s1, s2)
	}
	if s1 == s2 {
		t.Errorf("two draws identical: %q", s1)
	}
}

type failingEntropy struct{}

func (failingEntropy) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

// KLOG-013③：熵源失败必须响亮地 panic（kerror，可被 TryCatchRun 捕获），
// 而不是旧行为的 Warn + op(nil) 空指针崩在别处。
func TestGetRandom_SeedFailurePanicsLoudly(t *testing.T) {
	// 保存并清空单例状态，强制走播种路径
	oldRand := safeRand.seededRand
	oldReader := cryptoReader
	safeRand.seededRand = nil
	cryptoReader = failingEntropy{}
	defer func() {
		safeRand.seededRand = oldRand
		cryptoReader = oldReader
	}()

	ke := TryCatchRun(context.Background(), func() {
		RandomInt(context.Background(), 10)
	})
	if ke == nil {
		t.Fatal("seed failure must panic with kerror, got nil")
	}
	if ke.Type != "CryptoRandSeedFailed" {
		t.Errorf("kerror type = %s, want CryptoRandSeedFailed", ke.Type)
	}
}
