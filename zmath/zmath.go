package zmath

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
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
		//		zlog.Info("Sample:", index, count, oi)
		count = (1 - ratio)
		oi = index + 1
	}
	return
}

func Sigmoid(n float64) float64 {
	return 1 - (1 / (1 + math.Pow(math.E, n*8-4)))
}

// GetNiceDividesOf divides d into at most max parts.
// Good for dividing a graph into lines to show values on x and y-axis
func GetNiceDividesOf(d float64, max int, isMemory bool) float64 {
	l := math.Floor(math.Log10(d))
	n := math.Pow(10.0, l)
	if isMemory {
		n = math.Pow(1024.0, math.Ceil(l/3.0))
		for d/n > float64(max) {
			n = n * 2.0
		}
	}
	for d/n < float64(max) {
		n = n / 2.0
	}
	return n
}

// EaseInOut makes t "ease in" gradually at 0, and easy out, flattening off near 1.
// Good for animations
func EaseInOut(t float64) float64 {
	return 1 - math.Cos(t*math.Pi)
}

// for length items, IndexOfMostFrequent compares them with each other and finds which one is used the most.
// It returns the index to an aribitrary item which is used the most.
func IndexOfMostFrequent(length int, compare func(i, j int) bool) int {
	var cmax, imax int
	var counts = make([]int, length, length)
	for i := 0; i < length; i++ {
		for j := i + 1; j < length; j++ {
			if compare(i, j) {
				counts[i]++
				if counts[i] > cmax {
					cmax = counts[i]
					cmax = i
				}
			}
		}
	}
	return imax
}

// LengthIntoDividePoints finds the point that splits len in 2, then 2 points that split in 3, 4, etc, skipping ones already used.
// Final pass adds all not used yet.
func LengthIntoDividePoints(len int) (points []int) {
	start := time.Now()
	ln := len - 1
	l2 := len / 2
	set := make([]bool, len, len)
	points = make([]int, len, len)
	pi := 0
	for parts := 2; parts < l2; parts *= parts {
		for i := 1; i < parts; i++ {
			x := ln * i / parts
			if !set[x] {
				points[pi] = x
				pi++
				set[x] = true
			}
		}
	}
	for x := 0; x < len; x++ {
		if !set[x] {
			points[pi] = x
			pi++
			// set[x] = true -- we don't need to set it
		}
	}
	zlog.Info("Points:", len, time.Since(start))

	return
}

func RoundToMod(n, mod int64) int64 {
	return n - n%mod
}

// GetNextOfSliceCombinations takes n slices of values, and the current value of each one.
// It increments to the next value of first set, or 0th and next of second set etc.
// Returning all possible combinations of the slices, wrapping to start over again.
// The slices need to always be given with the same order.
func GetNextOfSliceCombinations[S comparable](sets [][]S, current ...*S) {
	indexes := make([]int, len(sets))
	for i, eachSet := range sets {
		for j, s := range eachSet {
			if s == *current[i] {
				indexes[i] = j
			}
		}
	}
	// zlog.Info("indexes:", indexes)
	for i := range indexes {
		top := i == len(sets)-1
		inc := indexes[i] < len(sets[i])-1
		if inc || top {
			indexes[i]++
			if top && !inc {
				// zlog.Info("top!", i, len(sets))
				indexes[i] = 0
			}
			for j := 0; j < i; j++ {
				indexes[j] = 0
			}
			break
		}
	}
	for i, index := range indexes {
		*current[i] = sets[i][index]
	}
}

func GetClosestTo(n float64, to []float64) float64 {
	best := -1
	for i, t := range to {
		a := math.Abs(n - t)
		if best == -1.0 || a < math.Abs(n-to[best]) {
			best = i
		}
	}
	return to[best]
}
