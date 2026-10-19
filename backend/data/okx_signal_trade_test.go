package data

import "testing"

func TestOKXSignalBarTimeWindows(t *testing.T) {
	const barTimeSeconds int64 = 1_797_056_123
	inputs := []int64{barTimeSeconds, barTimeSeconds * 1000}

	var want [4]int64
	for i, input := range inputs {
		msStart, msEnd, secStart, secEnd := okxSignalBarTimeWindows(input)
		got := [4]int64{msStart, msEnd, secStart, secEnd}
		if i == 0 {
			want = got
		} else if got != want {
			t.Fatalf("秒级与毫秒级 bar_time 区间不一致: %v != %v", got, want)
		}

		if input == barTimeSeconds {
			if secStart > input || input >= secEnd {
				t.Errorf("秒级时间戳 %d 不在秒级窗口 [%d,%d)", input, secStart, secEnd)
			}
		} else if msStart > input || input >= msEnd {
			t.Errorf("毫秒级时间戳 %d 不在毫秒级窗口 [%d,%d)", input, msStart, msEnd)
		}
		if secEnd-secStart != 60 || msEnd-msStart != 60_000 {
			t.Errorf("窗口不是一分钟: ms=[%d,%d) sec=[%d,%d)", msStart, msEnd, secStart, secEnd)
		}
	}
}
