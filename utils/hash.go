package utils

func GetHash(str string) uint32 {
	var hash1, hash2 uint32
	hash1 = 5381
	hash2 = hash1
	var c uint32
	s := []uint8(str)
	len := len(s)
	for i := 0; i < len; i += 2 {
		c = uint32(s[i])
		hash1 = ((hash1 << 5) + hash1) ^ c
		if i+1 >= len {
			break
		}
		c = uint32(s[i+1])
		hash2 = ((hash2 << 5) + hash2) ^ c
	}
	return hash1 + (hash2 * 1566083941)
}
