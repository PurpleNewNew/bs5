package neoreg

import (
	"math/big"
)

const (
	n         = 624
	m         = 397
	matrixA   = 0x9908b0df // 调用 MATRIX_A
	upperMask = 0x80000000 // UPPER_MASK
	lowerMask = 0x7fffffff // LOWER_MASK
)

// MT19937 封装了一个 32 位的 Mersenne Twister 状态，与 CPython random 模块对齐。
type MT19937 struct {
	state [n]uint32
	index int
}

// New 返回一个新的 MT19937 实例，默认用 seed=5489 初始化，
// 这是经典参考实现的默认做法（Python 中若不指定 seed，会从系统熵源播种）。
func NewMT19937() *MT19937 {
	mt := &MT19937{}
	mt.Seed(5489)
	return mt
}

func (mt *MT19937) SeedFromBigInt(bigSeed *big.Int) {
	mt.initGenRand(19650218)

	absSeed := new(big.Int).Abs(bigSeed)
	seedBytes := absSeed.Bytes() // big-endian
	if len(seedBytes) == 0 {
		seedBytes = []byte{0}
	}

	for i, j := 0, len(seedBytes)-1; i < j; i, j = i+1, j-1 {
		seedBytes[i], seedBytes[j] = seedBytes[j], seedBytes[i]
	}

	numChunks := (len(seedBytes) + 3) / 4
	key := make([]uint32, numChunks)
	for i := 0; i < numChunks; i++ {
		var chunk uint32
		for b := 0; b < 4; b++ {
			idx := 4*i + b
			if idx < len(seedBytes) {
				chunk |= uint32(seedBytes[idx]) << uint(8*b)
			}
		}
		key[i] = chunk
	}

	// init_by_array
	i, j := 1, 0
	k := len(key)
	if n > k {
		k = n
	}
	for ; k > 0; k-- {
		mt.state[i] = (mt.state[i] ^ ((mt.state[i-1] ^ (mt.state[i-1] >> 30)) * 1664525)) +
			key[j] + uint32(j)
		i++
		j++
		if i >= n {
			mt.state[0] = mt.state[n-1]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k = n - 1; k > 0; k-- {
		mt.state[i] = (mt.state[i] ^ ((mt.state[i-1] ^ (mt.state[i-1] >> 30)) * 1566083941)) -
			uint32(i)
		i++
		if i >= n {
			mt.state[0] = mt.state[n-1]
			i = 1
		}
	}
	mt.state[0] = 0x80000000
}

func (mt *MT19937) Seed(seed int64) {
	mt.SeedFromBigInt(big.NewInt(seed))
}

func (mt *MT19937) initGenRand(seed uint32) {
	mt.state[0] = seed
	for i := 1; i < n; i++ {
		// 这里的常数 1812433253U 与 Python 源码相同
		mt.state[i] = 1812433253*(mt.state[i-1]^(mt.state[i-1]>>30)) + uint32(i)
	}
	mt.index = n // 令其在下次取数时先进行一次 twist
}

// twist 完成一批新的 N 个随机数更新，与 CPython genrand_uint32() 里的批量更新一致。
func (mt *MT19937) twist() {
	for i := 0; i < n; i++ {
		// 将当前状态与下一状态组合出一个 32bit y
		y := (mt.state[i] & upperMask) | (mt.state[(i+1)%n] & lowerMask)
		// mt.state[i+(m)%n] 其实就是 mt.state[(i+m) % n]
		mt.state[i] = mt.state[(i+m)%n] ^ (y >> 1)
		if (y & 1) != 0 {
			mt.state[i] ^= matrixA
		}
	}
	mt.index = 0
}

func (mt *MT19937) Uint32() uint32 {
	if mt.index >= n {
		mt.twist()
	}
	y := mt.state[mt.index]
	mt.index++

	// tempering，与 CPython 源码完全一致
	y ^= (y >> 11)
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= (y >> 18)

	return y
}

// Float64 对应 CPython 的 random_random():
//
//	double random_random() {
//	    a = genrand_uint32() >> 5;  // 取高 27 bit
//	    b = genrand_uint32() >> 6;  // 取高 26 bit
//	    return (a * 67108864.0 + b) / 9007199254740992.0;  // 2^53 = 9007199254740992
//	}
func (mt *MT19937) Float64() float64 {
	a := mt.Uint32() >> 5 // 右移 5 得到高 27 位
	b := mt.Uint32() >> 6 // 右移 6 得到高 26 位
	return float64(a)*67108864.0 + float64(b)/float64(1<<53)
}

// GetRandBits(k) 模拟 Python getrandbits(k)，返回一个大整数。
func (mt *MT19937) GetRandBits(k int) *big.Int {
	if k < 0 {
		return big.NewInt(0)
	}
	if k == 0 {
		return big.NewInt(0)
	}

	words := (k + 31) / 32 // 向上取整
	var buf = make([]uint32, words)

	for i := 0; i < words; i++ {
		buf[i] = mt.Uint32()
	}

	lastBits := uint32(k % 32)
	if lastBits != 0 {
		shift := 32 - lastBits
		buf[words-1] >>= shift
	}

	bb := make([]byte, words*4)
	for i := 0; i < words; i++ {
		w := buf[i]
		bb[4*i+0] = byte(w)
		bb[4*i+1] = byte(w >> 8)
		bb[4*i+2] = byte(w >> 16)
		bb[4*i+3] = byte(w >> 24)
	}

	for i, j := 0, len(bb)-1; i < j; i, j = i+1, j-1 {
		bb[i], bb[j] = bb[j], bb[i]
	}

	res := new(big.Int).SetBytes(bb)
	return res
}
