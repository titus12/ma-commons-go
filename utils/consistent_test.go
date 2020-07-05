package utils

import (
	"testing"
)

func TestCopySlice(t *testing.T) {
	cHashRing := NewConsistent()

	cHashRing.Add(NewNodeKey("192.168.1.1:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.2:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.3:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.4:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.5:8080", 1))

	clone := cHashRing.Copy()


	clone.Add(NewNodeKey("111111:88888", 1))
	cHashRing.Remove("192.168.1.5:8080")


	t.Log(clone)
}

func TestConsistent1(t *testing.T) {
	cHashRing := NewConsistent()
	cHashRing.Add(NewNodeKey("192.168.1.1:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.2:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.3:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.4:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.5:8080", 1))

	n, err := cHashRing.Get("1001")
	if err != nil {
		return
	}
	t.Logf("nodes: %d-%v", len(cHashRing.Members()), n)

	cHashRing.Remove("192.168.1.1:8080")
	n, err = cHashRing.Get("1001")
	if err != nil {
		return
	}
	t.Logf("nodes: %d-%v", len(cHashRing.Members()), n)

	cHashRing.Remove("192.168.1.2:8080")
	n, err = cHashRing.Get("1001")
	if err != nil {
		return
	}
	t.Logf("nodes: %d-%v", len(cHashRing.Members()), n)

	cHashRing.Remove("192.168.1.3:8080")
	n, err = cHashRing.Get("1001")
	if err != nil {
		return
	}
	t.Logf("nodes: %d-%v", len(cHashRing.Members()), n)

	cHashRing.Remove("192.168.1.5:8080")

	n, err = cHashRing.Get("1001")
	if err != nil {
		return
	}
	t.Logf("nodes: %d-%v", len(cHashRing.Members()), n)

}

func TestConsistent_GetTwo(t *testing.T) {
	cHashRing := NewConsistent()
	cHashRing.Add(NewNodeKey("192.168.1.1:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.2:8080", 1))
	cHashRing.Add(NewNodeKey("192.168.1.3:8080", 1))

	users := []string{"user_mcnulty", "user_bunk", "user_omar", "user_bunny", "user_stringer"}
	t.Log("initial state [A, B, C]")

	for _, u := range users {
		server, err := cHashRing.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(server)
	}

	n1, n2, err := cHashRing.GetTwo("user_stringer")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(n1)
	t.Log(n2)
}
