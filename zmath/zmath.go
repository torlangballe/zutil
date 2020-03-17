package zmath

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/torlangballe/zutil/zint"
)

const MathDegreesToMeters = (111.32 * 1000)
const MathMetersToDegrees = 1 / MathDegreesToMeters

func RadToDeg(rad float64) float64 {
	return rad * 180 / math.Pi
}

func DegToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

func CalculateFraction(r float64, max int) (int, int) {
	var f, tBest, bBest, diffBest float64
	fmax := float64(max)
	diffBest = 10000.0
	for t := 0.0; t <= fmax; t++ {
		for b := 1.0; b <= fmax; b++ {
			f = t / b
			diff := math.Abs(f - r)
			if diff < diffBest {
				diffBest = diff
				tBest = t
				bBest = b
			}
			if f < r {
				break
			}
		}
		if f > r {
			break
		}
	}
	return int(tBest), int(bBest)
}

func CalculateFractionToString(r float64, max int, sep string) string {
	if r == -1.0 {
		return "-1"
	}
	top, bot := CalculateFraction(r, max)
	if top == 0 {
		return "0"
	}
	return fmt.Sprintf("%d%s%d", top, sep, bot)
}

func CalculateFractionFromString(str string, sep string) float64 {
	var top, bot float64
	if n, _ := fmt.Sscanf(str, "%f"+sep+"%f", &top, &bot); n == 2 {
		if bot == 0.0 {
			return 0
		}
		return top / bot
	}
	if n, _ := fmt.Sscanf(str, "%f", &top); n == 1 {
		return top
	}
	return 0.0
}

func Sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	if f > 0 {
		return 1
	}
	return 0
}

func RoundToAccuracy(f, accuracy float64) float64 { // this rounds to lower negative number)
	return math.Floor(f/accuracy) * accuracy //  + math.Copysign(accuracy/2, f)
}

func SinDeg(degrees float64) float64 {
	return math.Sin((degrees * math.Pi) / 180)
}

func CosDeg(degrees float64) float64 {
	return math.Cos((degrees * math.Pi) / 180)
}

func TanDeg(degrees float64) float64 {
	return math.Tan((degrees * math.Pi) / 180)
}

func AsinDeg(x float64) float64 {
	return math.Asin(x) * 180 / math.Pi
}

func AcosDeg(x float64) float64 {
	return math.Acos(x) * 180 / math.Pi
}

func AtanDeg(x float64) float64 {
	return math.Atan(x) * 180 / math.Pi
}

func UniqueRandomNumbersMapInRange(maxValue, count int) (nums map[int]bool) {
	nums = map[int]bool{}
	zint.Minimize(&count, maxValue)
	for len(nums) < count {
		nums[rand.Intn(maxValue)] = true
	}
	return
}

func RatioOfIntRandomRounded(ratio float32, count int) int {
	r := ratio * float32(count)
	i := int(r)
	if rand.Float32() <= r-float32(i) {
		i++
	}
	return i
}

func RoundDownWithRandom(c float32) int {
	i := int(c)
	if rand.Float32() <= c-float32(i) {
		i++
	}
	return i
}

// InterpolatedArrayRatioAtT returns what index in an array t (0-1) is.
// ratio is how much (0-1) of index value should be used and vs rest in next index value
func InterpolatedArrayRatioAtT(arrayLength int, t float64) (ratio float64, index int) {
	if t < 0.0 {
		return 1.0, 0
	}
	if t >= 1.0 {
		return 1.0, arrayLength - 1
	}
	f := float64(arrayLength-1) * t
	return 1 - (f - math.Floor(f)), int(f)
}

func Downsample(samples []float64, newSize int) (ns []float64) {
	slen := len(samples)
	var o float64
	var oi = -1
	ns = make([]float64, newSize, newSize)
	var count float64
	for i := 0; i < newSize; i++ {
		t := float64(i) / float64(newSize)
		ratio, index := InterpolatedArrayRatioAtT(slen, t)
		count += ratio
		o = samples[index] * ratio
		for j := oi + 1; j < index; j++ {
			o += samples[j]
			count++
		}
		ns[i] = o / count
		o = samples[index] * (1 - ratio)
		//		fmt.Println("Sample:", index, count, oi)
		count = (1 - ratio)
		oi = index + 1
	}
	return
}

func Sigmoid(n float64) float64 {
	return 1 - (1 / (1 + math.Pow(math.E, n*8-4)))
}

func GetNiceIncsOf(d float64, incCount int, isMemory bool) float64 {
	l := math.Floor(math.Log10(d))
	n := math.Pow(10.0, l)
	if isMemory {
		n = math.Pow(1024.0, math.Ceil(l/3.0))
		for d/n > float64(incCount) {
			n = n * 2.0
		}
	}
	for d/n < float64(incCount) {
		n = n / 2.0
	}
	return n
}
