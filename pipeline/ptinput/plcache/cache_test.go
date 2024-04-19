package plcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCache(t *testing.T) {
	_, err := NewCache(time.Second, 10)
	assert.Nil(t, err)
}

func TestCache_SetAndGet(t *testing.T) {
	cache, _ := NewCache(time.Second, 10)
	cache.Set("key1", "value1", 10*time.Second)
	cache.Set("key2", "value2", 10*time.Second)
	cache.Set("key3", "value3", 10*time.Second)
	cache.Set("key4", "value4", 10*time.Second)

	time.Sleep(2 * time.Second)

	v1, exists1, _ := cache.Get("key1")
	v2, exists2, _ := cache.Get("key2")
	v3, exists3, _ := cache.Get("key3")
	v4, exists4, _ := cache.Get("key4")

	t.Logf(v1.(string), v2.(string), v3.(string), v4.(string))
	assert.True(t, exists1 && exists2 && exists3 && exists4)
}

func TestCache_RemoveExpiredCache(t *testing.T) {
	cache, _ := NewCache(time.Second, 10)
	cache.Set("key", "value", 5*time.Second)

	time.Sleep(2 * time.Second)
	_, exist1, _ := cache.Get("key")
	time.Sleep(10 * time.Second)
	_, exist2, _ := cache.Get("key")

	assert.True(t, exist1 == true && exist2 == false)
}

func TestCache_Stop(t *testing.T) {
	cache, _ := NewCache(time.Second, 10)
	cache.Stop()

	err1 := cache.Set("key", "value", 10*time.Second)
	_, _, err2 := cache.Get("key")

	assert.NotNil(t, err1, "err1 is nil")
	assert.NotNil(t, err2, "err2 is nil")
}
