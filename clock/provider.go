//go:build !holster_test_mode

package clock

var (
	provider Clock = realtime
)

func setProvider(p Clock) {
	provider = p
}

func getProvider() Clock {
	return provider
}
