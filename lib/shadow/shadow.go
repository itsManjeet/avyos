/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package shadow authenticates users against /etc/shadow.
package shadow

import (
	"bufio"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const shadowPath = "/etc/shadow"

var (
	errUserNotFound    = errors.New("user not found")
	errAccountLocked   = errors.New("account is locked")
	errUnsupportedHash = errors.New("unsupported shadow hash")
)

type entry struct {
	name string
	hash string
}

// Hash returns a shadow-compatible password hash.
//
// Supported kinds:
//   - "", "sha512", "sha512-crypt", "crypt", "$6$": SHA-512 crypt ($6$)
//   - "legacy-sha512", "sha512-base64": legacy avyos sha512$base64 format
//   - "plain": plain:password development format
//   - "lock", "locked": locked account marker
func Hash(kind, password string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "plain":
		return "plain:" + password
	case "legacy-sha512", "sha512-base64":
		return legacySHA512Hash(password)
	case "lock", "locked":
		return "!"
	case "", "sha512", "sha512-crypt", "crypt", "$6$":
		hash, err := sha512Crypt(password, "$6$"+randomSalt(16)+"$")
		if err == nil {
			return hash
		}
		return legacySHA512Hash(password)
	default:
		hash, err := sha512Crypt(password, "$6$"+randomSalt(16)+"$")
		if err == nil {
			return hash
		}
		return legacySHA512Hash(password)
	}
}

// Authenticate verifies username/password against /etc/shadow.
func Authenticate(username, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, errUserNotFound
	}

	ent, err := lookup(username)
	if err != nil {
		return false, err
	}

	hash := strings.TrimSpace(ent.hash)
	if hash == "" {
		return password == "", nil
	}
	if isLocked(hash) {
		return false, errAccountLocked
	}

	return verify(password, hash)
}

func lookup(username string) (entry, error) {
	file, err := os.Open(shadowPath)
	if err != nil {
		return entry{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 2 || fields[0] != username {
			continue
		}
		return entry{name: fields[0], hash: fields[1]}, nil
	}
	if err := scanner.Err(); err != nil {
		return entry{}, err
	}
	return entry{}, errUserNotFound
}

func isLocked(hash string) bool {
	return strings.HasPrefix(hash, "!") || strings.HasPrefix(hash, "*")
}

func verify(password, hash string) (bool, error) {
	switch {
	case strings.HasPrefix(hash, "sha512$"):
		return verifyLegacySHA512(password, strings.TrimPrefix(hash, "sha512$")), nil
	case strings.HasPrefix(hash, "sha512:"):
		return verifyLegacySHA512(password, strings.TrimPrefix(hash, "sha512:")), nil
	case strings.HasPrefix(hash, "plain:"):
		want := strings.TrimPrefix(hash, "plain:")
		return subtle.ConstantTimeCompare([]byte(password), []byte(want)) == 1, nil
	case strings.HasPrefix(hash, "$6$"):
		computed, err := sha512Crypt(password, hash)
		if err != nil {
			return false, err
		}
		return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1, nil
	default:
		return false, errUnsupportedHash
	}
}

func verifyLegacySHA512(password, encoded string) bool {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}
	computed := sha512.Sum512([]byte(password))
	return subtle.ConstantTimeCompare(decoded, computed[:]) == 1
}

func legacySHA512Hash(password string) string {
	sum := sha512.Sum512([]byte(password))
	return "sha512$" + base64.StdEncoding.EncodeToString(sum[:])
}

func randomSalt(n int) string {
	const alphabet = cryptAlphabet
	if n <= 0 {
		n = 16
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		seed := sha512.Sum512([]byte(passwordlessSaltSeed()))
		copy(buf, seed[:])
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf)
}

func passwordlessSaltSeed() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36) + ":" + strconv.Itoa(os.Getpid())
}

func sha512Crypt(password, setting string) (string, error) {
	rounds := 5000
	rest := strings.TrimPrefix(setting, "$6$")
	prefix := "$6$"

	if after, ok := strings.CutPrefix(rest, "rounds="); ok {
		value, tail, ok := strings.Cut(after, "$")
		if !ok {
			return "", fmt.Errorf("%w: invalid rounds", errUnsupportedHash)
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("%w: invalid rounds", errUnsupportedHash)
		}
		if n < 1000 {
			n = 1000
		}
		if n > 999999999 {
			n = 999999999
		}
		rounds = n
		rest = tail
		prefix += "rounds=" + strconv.Itoa(rounds) + "$"
	}

	salt, _, _ := strings.Cut(rest, "$")
	if len(salt) > 16 {
		salt = salt[:16]
	}
	if salt == "" {
		return "", fmt.Errorf("%w: empty salt", errUnsupportedHash)
	}

	passwordBytes := []byte(password)
	saltBytes := []byte(salt)

	alt := sha512.New()
	alt.Write(passwordBytes)
	alt.Write(saltBytes)
	alt.Write(passwordBytes)
	altResult := alt.Sum(nil)

	ctx := sha512.New()
	ctx.Write(passwordBytes)
	ctx.Write(saltBytes)
	for i := len(passwordBytes); i > 0; i -= 64 {
		if i > 64 {
			ctx.Write(altResult)
		} else {
			ctx.Write(altResult[:i])
		}
	}
	for i := len(passwordBytes); i > 0; i >>= 1 {
		if i&1 == 1 {
			ctx.Write(altResult)
		} else {
			ctx.Write(passwordBytes)
		}
	}
	altResult = ctx.Sum(nil)

	pctx := sha512.New()
	for i := 0; i < len(passwordBytes); i++ {
		pctx.Write(passwordBytes)
	}
	pDigest := pctx.Sum(nil)
	pBytes := repeatToLength(pDigest, len(passwordBytes))

	sctx := sha512.New()
	for i := 0; i < 16+int(altResult[0]); i++ {
		sctx.Write(saltBytes)
	}
	sDigest := sctx.Sum(nil)
	sBytes := repeatToLength(sDigest, len(saltBytes))

	for i := 0; i < rounds; i++ {
		ctx = sha512.New()
		if i&1 == 1 {
			ctx.Write(pBytes)
		} else {
			ctx.Write(altResult)
		}
		if i%3 != 0 {
			ctx.Write(sBytes)
		}
		if i%7 != 0 {
			ctx.Write(pBytes)
		}
		if i&1 == 1 {
			ctx.Write(altResult)
		} else {
			ctx.Write(pBytes)
		}
		altResult = ctx.Sum(nil)
	}

	return prefix + salt + "$" + cryptBase64SHA512(altResult), nil
}

func repeatToLength(src []byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = src[i%len(src)]
	}
	return out
}

const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func cryptBase64SHA512(sum []byte) string {
	var b strings.Builder
	b.Grow(86)
	b64From24(&b, sum[0], sum[21], sum[42], 4)
	b64From24(&b, sum[22], sum[43], sum[1], 4)
	b64From24(&b, sum[44], sum[2], sum[23], 4)
	b64From24(&b, sum[3], sum[24], sum[45], 4)
	b64From24(&b, sum[25], sum[46], sum[4], 4)
	b64From24(&b, sum[47], sum[5], sum[26], 4)
	b64From24(&b, sum[6], sum[27], sum[48], 4)
	b64From24(&b, sum[28], sum[49], sum[7], 4)
	b64From24(&b, sum[50], sum[8], sum[29], 4)
	b64From24(&b, sum[9], sum[30], sum[51], 4)
	b64From24(&b, sum[31], sum[52], sum[10], 4)
	b64From24(&b, sum[53], sum[11], sum[32], 4)
	b64From24(&b, sum[12], sum[33], sum[54], 4)
	b64From24(&b, sum[34], sum[55], sum[13], 4)
	b64From24(&b, sum[56], sum[14], sum[35], 4)
	b64From24(&b, sum[15], sum[36], sum[57], 4)
	b64From24(&b, sum[37], sum[58], sum[16], 4)
	b64From24(&b, sum[59], sum[17], sum[38], 4)
	b64From24(&b, sum[18], sum[39], sum[60], 4)
	b64From24(&b, sum[40], sum[61], sum[19], 4)
	b64From24(&b, sum[62], sum[20], sum[41], 4)
	b64From24(&b, 0, 0, sum[63], 2)
	return b.String()
}

func b64From24(b *strings.Builder, b2, b1, b0 byte, n int) {
	w := uint32(b2)<<16 | uint32(b1)<<8 | uint32(b0)
	for n > 0 {
		b.WriteByte(cryptAlphabet[w&0x3f])
		w >>= 6
		n--
	}
}
