package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

var (
	testSnapshotVmStr = `
{
  "snapshot_id": "RCJE706V",
  "all_shards": [
    {
      "shard_id": "shard_00_08",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "6L9I4UQ7"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_08_10",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "WTWGRM94"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_10_18",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "XF2FA7SJ"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_18_20",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "6LDMUFDL"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_20_28",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "3BNAW9JP"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_28_30",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "Q0HZFLL2"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_30_38",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "SSYNOQCY"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_38_40",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "X78YTZTA"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_40_48",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "WNLFQ8TC"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_48_50",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "D89CE3FA"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_50_58",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "2UNYGWAT"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_58_60",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "S9406O5W"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_60_68",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "Z37SZUPO"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_68_70",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "Z5FXHCEA"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_70_78",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "JST18XDC"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_78_80",
      "target_count": 1,
      "replicas": [
        {
          "idx": 3,
          "assigns": [
            "44LNYGY2"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_80_88",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "0RN627CS"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_88_90",
      "target_count": 1,
      "replicas": [
        {
          "idx": 2,
          "assigns": [
            "WXWLP155"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_90_98",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "03EQSP0Q"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_98_a0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "VWDBYHTA"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_a0_a8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "9BXFW86W"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_a8_b0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "DNO5C7WS"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_b0_b8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "GJNMUHJ5"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_b8_c0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "OC6JOKWI"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_c0_c8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "0A0OD5CI"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_c8_d0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "55UOMLKA"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_d0_d8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "6A7BDP6O"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_d8_e0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 2,
          "assigns": [
            "8D02BW1Q"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_e0_e8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "RXK1W8IQ"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_e8_f0",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "7VNY7S9I"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_f0_f8",
      "target_count": 1,
      "replicas": [
        {
          "idx": 0,
          "assigns": [
            "9WRANCC2"
          ]
        }
      ]
    },
    {
      "shard_id": "shard_f8_00",
      "target_count": 1,
      "replicas": [
        {
          "idx": 1,
          "assigns": [
            "XB0VGHSB"
          ]
        }
      ]
    }
  ],
  "all_workers": [
    {
      "worker_id": "smgapp-c7d544b97-2pv22",
      "session_id": "EQ3OB19S",
      "assigns": [
        {
          "sid": "shard_00_08",
          "aid": "6L9I4UQ7"
        },
        {
          "sid": "shard_40_48",
          "aid": "WNLFQ8TC"
        },
        {
          "sid": "shard_70_78",
          "aid": "JST18XDC"
        },
        {
          "sid": "shard_c8_d0",
          "aid": "55UOMLKA"
        },
        {
          "sid": "shard_e0_e8",
          "aid": "RXK1W8IQ"
        }
      ]
    },
    {
      "worker_id": "smgapp-c7d544b97-86g6q",
      "session_id": "VKXMD7G6",
      "assigns": [
        {
          "sid": "shard_78_80",
          "aid": "44LNYGY2"
        },
        {
          "sid": "shard_90_98",
          "aid": "03EQSP0Q"
        },
        {
          "sid": "shard_a8_b0",
          "aid": "DNO5C7WS"
        },
        {
          "sid": "shard_b0_b8",
          "aid": "GJNMUHJ5"
        },
        {
          "sid": "shard_f0_f8",
          "aid": "9WRANCC2"
        }
      ]
    },
    {
      "worker_id": "smgapp-c7d544b97-bhgcw",
      "session_id": "BAFKEUFJ",
      "assigns": [
        {
          "sid": "shard_10_18",
          "aid": "XF2FA7SJ"
        },
        {
          "sid": "shard_20_28",
          "aid": "3BNAW9JP"
        },
        {
          "sid": "shard_30_38",
          "aid": "SSYNOQCY"
        },
        {
          "sid": "shard_88_90",
          "aid": "WXWLP155"
        },
        {
          "sid": "shard_b8_c0",
          "aid": "OC6JOKWI"
        },
        {
          "sid": "shard_d8_e0",
          "aid": "8D02BW1Q"
        }
      ]
    },
    {
      "worker_id": "smgapp-c7d544b97-m6z65",
      "session_id": "X45O2XGQ",
      "assigns": [
        {
          "sid": "shard_50_58",
          "aid": "2UNYGWAT"
        },
        {
          "sid": "shard_58_60",
          "aid": "S9406O5W"
        },
        {
          "sid": "shard_60_68",
          "aid": "Z37SZUPO"
        },
        {
          "sid": "shard_68_70",
          "aid": "Z5FXHCEA"
        },
        {
          "sid": "shard_e8_f0",
          "aid": "7VNY7S9I"
        },
        {
          "sid": "shard_f8_00",
          "aid": "XB0VGHSB"
        }
      ]
    },
    {
      "worker_id": "smgapp-c7d544b97-mcc27",
      "session_id": "SAA1GSGF",
      "assigns": [
        {
          "sid": "shard_18_20",
          "aid": "6LDMUFDL"
        },
        {
          "sid": "shard_48_50",
          "aid": "D89CE3FA"
        },
        {
          "sid": "shard_98_a0",
          "aid": "VWDBYHTA"
        },
        {
          "sid": "shard_a0_a8",
          "aid": "9BXFW86W"
        },
        {
          "sid": "shard_d0_d8",
          "aid": "6A7BDP6O"
        }
      ]
    },
    {
      "worker_id": "smgapp-c7d544b97-mfx58",
      "session_id": "HKV6XKIL",
      "assigns": [
        {
          "sid": "shard_08_10",
          "aid": "WTWGRM94"
        },
        {
          "sid": "shard_28_30",
          "aid": "Q0HZFLL2"
        },
        {
          "sid": "shard_38_40",
          "aid": "X78YTZTA"
        },
        {
          "sid": "shard_80_88",
          "aid": "0RN627CS"
        },
        {
          "sid": "shard_c0_c8",
          "aid": "0A0OD5CI"
        }
      ]
    }
  ],
  "all_assignments": [
    {
      "aid": "6L9I4UQ7",
      "sid": "shard_00_08",
      "wid": "smgapp-c7d544b97-2pv22",
      "ses_id": "EQ3OB19S"
    },
    {
      "aid": "WTWGRM94",
      "sid": "shard_08_10",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mfx58",
      "ses_id": "HKV6XKIL"
    },
    {
      "aid": "XF2FA7SJ",
      "sid": "shard_10_18",
      "idx": 1,
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "6LDMUFDL",
      "sid": "shard_18_20",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mcc27",
      "ses_id": "SAA1GSGF"
    },
    {
      "aid": "3BNAW9JP",
      "sid": "shard_20_28",
      "idx": 1,
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "Q0HZFLL2",
      "sid": "shard_28_30",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mfx58",
      "ses_id": "HKV6XKIL"
    },
    {
      "aid": "SSYNOQCY",
      "sid": "shard_30_38",
      "idx": 1,
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "X78YTZTA",
      "sid": "shard_38_40",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mfx58",
      "ses_id": "HKV6XKIL"
    },
    {
      "aid": "WNLFQ8TC",
      "sid": "shard_40_48",
      "wid": "smgapp-c7d544b97-2pv22",
      "ses_id": "EQ3OB19S"
    },
    {
      "aid": "D89CE3FA",
      "sid": "shard_48_50",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mcc27",
      "ses_id": "SAA1GSGF"
    },
    {
      "aid": "2UNYGWAT",
      "sid": "shard_50_58",
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    },
    {
      "aid": "S9406O5W",
      "sid": "shard_58_60",
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    },
    {
      "aid": "Z37SZUPO",
      "sid": "shard_60_68",
      "idx": 1,
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    },
    {
      "aid": "Z5FXHCEA",
      "sid": "shard_68_70",
      "idx": 1,
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    },
    {
      "aid": "JST18XDC",
      "sid": "shard_70_78",
      "wid": "smgapp-c7d544b97-2pv22",
      "ses_id": "EQ3OB19S"
    },
    {
      "aid": "44LNYGY2",
      "sid": "shard_78_80",
      "idx": 3,
      "wid": "smgapp-c7d544b97-86g6q",
      "ses_id": "VKXMD7G6"
    },
    {
      "aid": "0RN627CS",
      "sid": "shard_80_88",
      "wid": "smgapp-c7d544b97-mfx58",
      "ses_id": "HKV6XKIL"
    },
    {
      "aid": "WXWLP155",
      "sid": "shard_88_90",
      "idx": 2,
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "03EQSP0Q",
      "sid": "shard_90_98",
      "idx": 1,
      "wid": "smgapp-c7d544b97-86g6q",
      "ses_id": "VKXMD7G6"
    },
    {
      "aid": "VWDBYHTA",
      "sid": "shard_98_a0",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mcc27",
      "ses_id": "SAA1GSGF"
    },
    {
      "aid": "9BXFW86W",
      "sid": "shard_a0_a8",
      "wid": "smgapp-c7d544b97-mcc27",
      "ses_id": "SAA1GSGF"
    },
    {
      "aid": "DNO5C7WS",
      "sid": "shard_a8_b0",
      "idx": 1,
      "wid": "smgapp-c7d544b97-86g6q",
      "ses_id": "VKXMD7G6"
    },
    {
      "aid": "GJNMUHJ5",
      "sid": "shard_b0_b8",
      "wid": "smgapp-c7d544b97-86g6q",
      "ses_id": "VKXMD7G6"
    },
    {
      "aid": "OC6JOKWI",
      "sid": "shard_b8_c0",
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "0A0OD5CI",
      "sid": "shard_c0_c8",
      "idx": 1,
      "wid": "smgapp-c7d544b97-mfx58",
      "ses_id": "HKV6XKIL"
    },
    {
      "aid": "55UOMLKA",
      "sid": "shard_c8_d0",
      "wid": "smgapp-c7d544b97-2pv22",
      "ses_id": "EQ3OB19S"
    },
    {
      "aid": "6A7BDP6O",
      "sid": "shard_d0_d8",
      "wid": "smgapp-c7d544b97-mcc27",
      "ses_id": "SAA1GSGF"
    },
    {
      "aid": "8D02BW1Q",
      "sid": "shard_d8_e0",
      "idx": 2,
      "wid": "smgapp-c7d544b97-bhgcw",
      "ses_id": "BAFKEUFJ"
    },
    {
      "aid": "RXK1W8IQ",
      "sid": "shard_e0_e8",
      "wid": "smgapp-c7d544b97-2pv22",
      "ses_id": "EQ3OB19S"
    },
    {
      "aid": "7VNY7S9I",
      "sid": "shard_e8_f0",
      "idx": 1,
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    },
    {
      "aid": "9WRANCC2",
      "sid": "shard_f0_f8",
      "wid": "smgapp-c7d544b97-86g6q",
      "ses_id": "VKXMD7G6"
    },
    {
      "aid": "XB0VGHSB",
      "sid": "shard_f8_00",
      "idx": 1,
      "wid": "smgapp-c7d544b97-m6z65",
      "ses_id": "X45O2XGQ"
    }
  ]
}
`
)

func TestSnapshotVmFromJson(t *testing.T) {
	ctx := context.Background()
	snapshotVm := ParseSnapshotVmFromJson(testSnapshotVmStr)
	if snapshotVm == nil {
		t.Fatal("Failed to parse snapshot VM from JSON")
	}

	// 验证快照ID
	assert.Equal(t, SnapshotId("RCJE706V"), snapshotVm.SnapshotId, "SnapshotId should be 'RCJE706V'")
	// 验证工作节点数量
	assert.Len(t, snapshotVm.AllWorkers, 6, "There should be 6 workers in the snapshot VM")
	// 验证分配数量
	assert.Len(t, snapshotVm.AllAssignments, 32, "There should be 32 assignments in the snapshot VM")
	// 验证分片数量
	assert.Len(t, snapshotVm.AllShards, 32, "There should be 32 shards in the snapshot VM")

	snapshot := SnapshotFromVm(ctx, snapshotVm, config.CostFuncConfigJsonToConfig(nil))
	assert.NotNil(t, snapshot, "Snapshot should not be nil")

	// cost
	cost := snapshot.GetCost(ctx)
	assert.Equal(t, int32(0), cost.HardScore, "HardScore should be 0")

	// test a move?
	{
		move := NewUnassignMove(data.WorkerFullIdParseFromString("smgapp-c7d544b97-m6z65:X45O2XGQ"), data.ReplicaFullIdParseFromString("shard_f8_00:1"), data.AssignmentId("XB0VGHSB"))
		snapshot2 := snapshot.Clone()
		snapshot2.ApplyMove(move, AM_Strict)
		cost2 := snapshot2.GetCost(ctx)
		assert.Equal(t, int32(2), cost2.HardScore, "HardScore should still be 2 after unassigning")
	}
}
