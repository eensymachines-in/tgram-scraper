package tokens_test

import (
	"fmt"
	"testing"

	"github.com/eensymachines/tgramscraper/tokens"
	"github.com/stretchr/testify/assert"
)

func ExampleNewSimpleTokenRegistry() {
	reg := tokens.NewSimpleTokenRegistry("6425245255:EGyHrU-i9MjCL5ZiTBl9k33UBH-o51-G5g4")
	fmt.Println(reg)
	// Output:
	// &{map[6425245255:6425245255:EGyHrU-i9MjCL5ZiTBl9k33UBH-o51-G5g4]}
}
func ExampleNewSimpleTokenRegistry_badToken() {
	// Observe that token has id that is alphanumeric while its expected to have only numberic
	reg := tokens.NewSimpleTokenRegistry("64252452fff:EGyHrU-i9MjCL5ZiTBl9k33UBH-o51-G5g4")
	fmt.Println(reg)
	// Another case when the token is empty
	reg = tokens.NewSimpleTokenRegistry("")
	fmt.Println(reg)
	// Another case when the token does not conform to the pattern
	// Notice how the ":" is missing
	reg = tokens.NewSimpleTokenRegistry("64252452fffEGyHrU-i9MjCL5ZiTBl9k33UBH-o51-G5g4")
	fmt.Println(reg)
	// Output:
	// &{map[]}
	// &{map[]}
	// &{map[]}
}
func TestCreateRegistry(t *testing.T) {
	// TEST: creating a new registry
	cases := []string{
		"6425245255:EGyHrU-i9MjCL5ZiTBl9k33UBH-o51-G5g4",
		"6214446136:oOkCGb-FjTX43v4u4A2p1IOED0-oHZ-hMPt",
		"7679837037:aePQBm-7cABKvZ7sOG6l1q21ha-5NB-2Sj2",
		"0895073343:udYegH-R3CyD8PH2BqPKhxBVvY-qez-x9u4",
		"1551601961:YqHlSS-3K7DCa1pevjzs6Ix9a7-w2I-B4P1",
		"3682856461:ftgowf-628E8q0TYqD79F0u72B-IHQ-h2gy",

		"0012817082:abVdHd-aTQ12Ynmr5iAj0rbYyd-4iI-FB62",
		"3364339064:gwZGqH-xxPS6d4jK9m769I3qcV-1FF-e6Fn",
		"7738711252:mrHeAO-i5kq4I7WaV65r3s3p6j-u2n-p3dV",
		"3226251931:dyaDVx-XRpZXbKQnTR7HRJwA7B-g7b-eMiB",
		"3226251931:dyaDVx-XRpZXbKQnTR7HRJwA7B-g7b-eMiB", // duplicate registry: will not be in the registry
	}

	registry := tokens.NewSimpleTokenRegistry(cases...)
	assert.NotEqual(t, 0, registry.Count(), "Count of registered cannot be zero")
	t.Logf("number of registered bots %d", registry.Count())

	// TEST: getting a few of the tokens from the registered ones
	token, ok := registry.Find("0012817082")
	assert.NotEqual(t, "", token, "Unexpected empty token retrieved")
	assert.Equal(t, true, ok, "Unexpected false value on the return flag")

	// TEST: getting a few of the tokens from the registered ones

	token, ok = registry.Find("8153206279")
	assert.Equal(t, "", token, "Unexpected non empty token retrieved")
	assert.Equal(t, false, ok, "Unexpected true value on the return flag")

	// TEST: all the invalid tokens
	notOkToks := []string{ // invalid tokens : none of this can be stored in the registry
		"",
		" ",
		"3682856461",
		"::",
		// alien strings from mockaroo
		"¸˛Ç◊ı˜Â¯˘¿",
		"Z̮̞̠͙͔ͅḀ̗̞͈̻̗Ḷ͙͎̯̹̞͓G̻O̭̗̮",
		"사회과학원 어학연구소",
		"-1/2",
		"NULL",
	}
	registry = tokens.NewSimpleTokenRegistry(notOkToks...)
	assert.Equal(t, 0, registry.Count(), "Unexpected non zero count of registeries")

}
