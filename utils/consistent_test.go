package utils

import (
	"fmt"
	"log"
	"testing"
)

func TestConsistent_GetTwo(t *testing.T) {
	cHashRing := NewConsistent()
	cHashRing.Add(NewNode("192.168.1.1", 8080, "host_1", 1))
	cHashRing.Add(NewNode("192.168.1.2", 8080, "host_2", 1))
	cHashRing.Add(NewNode("192.168.1.3", 8080, "host_3", 1))

	users := []string{"user_mcnulty", "user_bunk", "user_omar", "user_bunny", "user_stringer"}
	t.Log("initial state [A, B, C]")

	for _, u := range users {
		server, err := cHashRing.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("\t%s => %s\n", u, server.Ip())
	}

	n1, n2, err := cHashRing.GetTwo("user_stringer")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(n1)
	t.Log(n2)
}

func TestNewConsistent2(t *testing.T) {
	cHashRing := NewConsistent()
	for i := 0; i < 10; i++ {
		si := fmt.Sprintf("%d", i)
		cHashRing.Add(NewNode("172.18.1."+si, 8080, "host_"+si, 1))
	}

	ipMap := make(map[string]int, 0)
	for i := 0; i < 100; i++ {
		si := fmt.Sprintf("key%d", i)
		k, _ := cHashRing.Get(si)

		if _, ok := ipMap[k.Ip()]; ok {
			ipMap[k.Ip()] += 1
		} else {
			ipMap[k.Ip()] = 1
		}
	}

	for k, v := range ipMap {
		t.Logf("Node IP: %s, COUNT: %d", k, v)
	}
}

func TestNewConsistent(t *testing.T) {
	cHashRing := NewConsistent()
	cHashRing.Add(NewNode("192.168.1.1", 8080, "host_1", 1))
	cHashRing.Add(NewNode("192.168.1.2", 8080, "host_2", 1))
	cHashRing.Add(NewNode("192.168.1.3", 8080, "host_3", 1))
	//for i := 0; i < 3; i++ {
	//	si := fmt.Sprintf("%d", i)
	//	cHashRing.Add(NewNode("172.18.1."+si, 8080, "host_"+si, 1))
	//}

	users := []string{"user_mcnulty", "user_bunk", "user_omar", "user_bunny", "user_stringer"}
	t.Log("initial state [A, B, C]")

	for _, u := range users {
		server, err := cHashRing.Get(u)
		if err != nil {
			log.Fatal(err)
		}
		t.Logf("\t%s => %s\n", u, server.Ip())
	}

	cHashRing.Add(NewNode("192.168.1.4", 8080, "host_4", 1))
	cHashRing.Add(NewNode("192.168.1.5", 8080, "host_5", 1))

	t.Log("with cacheD, cacheE [A, B, C, D, E]")

	for _, u := range users {
		server, err := cHashRing.Get(u)
		if err != nil {
			log.Fatal(err)
		}
		t.Logf("\t%s => %s\n", u, server.Ip())
	}

	//ipMap := make(map[string]int, 0)
	//for i := 0; i < 10; i++ {
	//	si := fmt.Sprintf("key%d", i)
	//	k, _ := cHashRing.Get(si)
	//
	//	if _, ok := ipMap[k.Ip()]; ok {
	//		ipMap[k.Ip()] += 1
	//	} else {
	//		ipMap[k.Ip()] = 1
	//	}
	//}

	//for k, v := range ipMap {
	//	t.Logf("Node IP: %s, COUNT: %d", k, v)
	//}
}
