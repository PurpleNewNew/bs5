package neoreg

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"math/rand"
	"strings"
)

type NeoregRand struct {
	mt     *MT19937
	v_clen *big.Int
}

// NewRandGenerator 创建 NeoregRand 实例
func NewNeoregRand(key string) *NeoregRand {
	rg := &NeoregRand{
		mt: NewMT19937(),
	}

	keyMinLen := 28

	var keyHash string
	if len(key) < keyMinLen {
		head := saltPrefix[:keyMinLen]
		tail := saltPrefix[keyMinLen:]
		sum := md5.Sum(append(append([]byte{}, head...), append([]byte(key), tail...)...))
		keyHash = hex.EncodeToString(sum[:])
	} else {
		keyHash = key
	}

	sub := keyHash
	if len(sub) > keyMinLen {
		sub = sub[:keyMinLen]
	}
	subBytes := []byte(sub) // ASCII
	hexEncoded := make([]byte, hex.EncodedLen(len(subBytes)))
	hex.Encode(hexEncoded, subBytes) // 变成十六进制 ASCII
	nBig := new(big.Int)
	nBig.SetString(string(hexEncoded), 16) // base 16 解析

	eStr := saltPrefix[:keyMinLen]
	mStr := saltPrefix[keyMinLen:]
	eBig := base36ToBig(eStr)
	mBig := base36ToBig(mStr)

	vClen := new(big.Int).Exp(nBig, eBig, mBig)
	rg.v_clen = vClen

	// 4) random.seed(n) => Go 里用 mt.SeedFromBigInt(nBig)
	rg.mt.SeedFromBigInt(nBig)

	return rg
}

func (rg *NeoregRand) RandValue() string {
	// bits
	bits := int(rg.mt.Float64()*300) + 30
	// getrandbits
	randBits := rg.mt.GetRandBits(bits)

	// val = (randBits << 280) + v_clen
	val := new(big.Int).Lsh(randBits, 280)
	val.Add(val, rg.v_clen)

	// base64
	raw := val.Bytes() // big-endian
	b64 := base64.StdEncoding.EncodeToString(raw)
	// 去掉 '='
	b64 = strings.TrimRight(b64, "=")
	return b64
}

// Base64Chars 模拟 Python 的 shuffle
func (rg *NeoregRand) Base64Chars(chars []rune) {
	// mimic python's shuffle which uses _randbelow
	for i := len(chars) - 1; i > 0; i-- {
		j := rg.randBelow(i + 1)
		chars[i], chars[j] = chars[j], chars[i]
	}
}

// randBelow 模拟 Python 的 _randbelow(n)
func (rg *NeoregRand) randBelow(n int) int {
	if n <= 1 {
		return 0
	}
	nBig := big.NewInt(int64(n))
	limitBits := nBig.BitLen()

	for {
		candidate := rg.mt.GetRandBits(limitBits)
		if candidate.Cmp(nBig) < 0 {
			return int(candidate.Int64()) // n 不大时可安全转换
		}
	}
}

// base36ToBig 把一段字节按 base36 转成大整数
func base36ToBig(b []byte) *big.Int {
	// Go 没有内置 base36，需要手写或一次性 parse
	// 这里简单手写 parse
	res := new(big.Int)
	for _, ch := range b {
		res.Mul(res, big.NewInt(36))
		var val int
		switch {
		case ch >= '0' && ch <= '9':
			val = int(ch - '0')
		case ch >= 'a' && ch <= 'z':
			val = int(ch - 'a' + 10)
		case ch >= 'A' && ch <= 'Z':
			val = int(ch - 'A' + 10)
		default:
			val = 0
		}
		res.Add(res, big.NewInt(int64(val)))
	}
	return res
}

func randbyte() []byte {
	min := 5
	max := 20
	length := rand.Intn(max-min-1) + 1
	data := make([]byte, length)
	rand.Read(data)
	return data
}

func randMask() []byte {
	data := make([]byte, 4)
	rand.Read(data)
	return []byte(hex.EncodeToString(data))
}
