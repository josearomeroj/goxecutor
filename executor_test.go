package goxecutor

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestNewFixedPoolExecutor(t *testing.T) {
	t.Parallel()

	workers := getRandomRange()
	e := NewFixedPoolExecutor(uint32(workers))

	arrayTest(e, int(workers), t)
}

func TestNewSingleThreadExecutor(t *testing.T) {
	t.Parallel()

	num := getRandomRange()
	e := NewSingleThreadExecutor()

	arrayTest(e, int(num), t)
}

func arrayTest(e Executor, num int, t *testing.T) {
	arr := make([]int, num)

	for i := 0; i < num; i++ {
		temp := i
		e.Submit(func() {
			arr[temp] = temp + 1
		})
	}

	e.Wait()

	for i := 0; i < num; i++ {
		assert.Equalf(t, i+1, arr[i], "num: %v", num)
	}
}

func TestExecutor_Destroy(t *testing.T) {
	t.Parallel()

	workers := getRandomRange()

	e := NewFixedPoolExecutor(uint32(workers))

	arr := make([]int, 0)
	for i := 0; i < int(workers); i++ {
		e.Submit(func() {
			arr = append(arr)
		})
	}

	e.Destroy()
	e.Wait()

	defer func() {
		r := recover()
		assert.NotNilf(t, r, "code did not panick")
	}()

	e.Submit(func() {
		print("this submit cannot be made")
	})
}

func TestExecutor_SubmitWithPriority(t *testing.T) {
	t.Parallel()

	e := NewSingleThreadExecutor()
	e.Submit(func() {
		time.Sleep(2 * time.Second)
	})

	arr := make([]int32, 3)
	i := 0

	noPriority := rand.Int31()
	priority := rand.Int31()
	morePriority := rand.Int31()

	e.Submit(func() {
		arr[i] = noPriority
		i++
	})
	e.SubmitWithPriority(func() {
		arr[i] = morePriority
		i++
	}, 2)
	e.SubmitWithPriority(func() {
		arr[i] = priority
		i++
	}, 1)

	e.Wait()

	assert.Equal(t, arr[0], morePriority)
	assert.Equal(t, arr[1], priority)
	assert.Equal(t, arr[2], noPriority)
}

func getRandomRange() int32 {
	return rand.Int31n(100-1) + 1
}
