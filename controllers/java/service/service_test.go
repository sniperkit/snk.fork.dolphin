/*
Sniperkit-Bot
- Status: analyzed
*/

package service

import (
	"fmt"
	"math/rand"
	"testing"
)

func mean(d []float64) float64 {
	l := len(d)
	s := .0
	for _, v := range d {
		s += v
	}
	return s / float64(l)
}

func Test_calFailRatio(t *testing.T) {
	size := 15 * 12 * 10
	arr := make([]float64, size)
	for i := 0; i < size; i++ {
		n := rand.Intn(100)
		if n > 90 {
			arr[i] = 0.0
		} else {
			arr[i] = 1.0
		}
		//arr[i] = float64(rand.Intn(2))
		//arr[i] = float64(1)
	}

	initVal := 0.0
	f1 := initVal
	f5 := initVal
	f15 := initVal
	for i, v := range arr {
		f1 = calFailRatio(f1, exp1, v)
		f5 = calFailRatio(f5, exp5, v)
		f15 = calFailRatio(f15, exp15, v)
		s1 := i + 1 - 12
		s2 := i + 1 - 12*5
		if s1 < 0 {
			s1 = 0
		}
		if s2 < 0 {
			s2 = 0
		}

		fmt.Printf("%d: %v, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f\n", i, v, f1, mean(arr[s1:i+1]), f5, mean(arr[s2:i+1]), f15, mean(arr[:i+1]))
	}
}
