package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	sdkkey "github.com/binance-chain/go-sdk/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/bepswap/thornode/cmd"
)

const baseTssPort = 8080

type Status byte

// KeySignResp key sign response
type KeySignResp struct {
	R      string `json:"r"`
	S      string `json:"s"`
	Status Status `json:"status"`
}

// KeySignReq request to sign a message
type KeySignReq struct {
	PoolPubKey string `json:"pool_pub_key"` // pub key of the pool that we would like to send this message from
	Message    string `json:"message"`      // base64 encoded message to be signed
}

type KeyGenResp struct {
	PubKey      string   `json:"pub_key"`
	PoolAddress string   `json:"pool_address"`
	Status      Status   `json:"status"`
	FailReason  string   `json:"fail_reason"`
	Blame       []string `json:"blame_peers"`
}

// KeyGenReq request to do keygen
type KeyGenReq struct {
	Keys []string `json:"keys"`
}

func sendTestRequest(url string, request []byte) []byte {
	var resp *http.Response
	var err error
	fmt.Println(url)
	if len(request) == 0 {
		resp, err = http.Get(url)
		if err != nil{
			fmt.Println(err)
		}
	} else {
		resp, err = http.Post(url, "application/json", bytes.NewBuffer(request))
		if err != nil{
			fmt.Println(err)
		}
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error!!!!!!!!!!+%v", err)
	}
	return body
}

func testKeySign(poolPubKey string, ips []string) {
	var keySignRespArr []*KeySignResp
	var locker sync.Mutex
	msg := base64.StdEncoding.EncodeToString([]byte("hello"))
	keySignReq := KeySignReq{
		PoolPubKey: poolPubKey,
		Message:    msg,
	}
	request, err := json.Marshal(keySignReq)
	requestGroup := sync.WaitGroup{}
	for i := 0; i < len(ips); i++ {
		requestGroup.Add(1)
		go func(i int, request []byte, keySignRespArr *[]*KeySignResp, locker *sync.Mutex) {
			defer requestGroup.Done()
			url := fmt.Sprintf("http://%s:%d/keysign", ips[i], baseTssPort)
			respByte := sendTestRequest(url, request)
			var tempResp KeySignResp
			err = json.Unmarshal(respByte, &tempResp)
			if err != nil {
				fmt.Printf("22222>>>>>err%v\n", err)

			}
			locker.Lock()
			*keySignRespArr = append(*keySignRespArr, &tempResp)
			locker.Unlock()
		}(i, request, &keySignRespArr, &locker)
}
	requestGroup.Wait()
	//this first node should get the empty result
	// size of the signature should be 44
	for i := 1; i < len(ips); i++ {
		fmt.Printf("%d----->%v\n", i,keySignRespArr[i].S)
	}
}

func testKeyGen(testPubKeys []string, ips []string) string {
	var keyGenRespArr []*KeyGenResp
	var locker sync.Mutex
	keyGenReq := KeyGenReq{
		Keys: testPubKeys[:],
	}
	request, err := json.Marshal(keyGenReq)
	if err != nil {
		return ""
	}

	requestGroup := sync.WaitGroup{}

	for i := 0; i < len(ips); i++ {

		requestGroup.Add(1)
		go func(i int, request []byte, keygenRespAddr *[]*KeyGenResp, locker *sync.Mutex) {
			defer requestGroup.Done()
			url := fmt.Sprintf("http://%s:%d/keygen", ips[i], baseTssPort)
			respByte := sendTestRequest(url, request)
			var tempResp KeyGenResp
			err = json.Unmarshal(respByte, &tempResp)
			if err != nil {
				fmt.Printf("22222>>>>>err%v\n", err)

			}
			locker.Lock()
			*keygenRespAddr = append(*keygenRespAddr, &tempResp)
			locker.Unlock()
		}(i, request, &keyGenRespArr, &locker)

	}

	requestGroup.Wait()

	for i := 0; i < len(ips); i++ {
		fmt.Printf("%d------%s\n", i, keyGenRespArr[i].PubKey)
	}
	return keyGenRespArr[0].PubKey
}

func docommand(ip, remoteFile, filePath, pemLocation string, send bool) {
	var out bytes.Buffer
	cmd := exec.Command("bash")
	cmdWriter, _ := cmd.StdinPipe()
	cmd.Start()
	cmd.Stdout = &out
	cmdString := ""
	if send {
		cmdString = fmt.Sprintf("scp -i %s %s %s@%s:%s", pemLocation, filePath, "ubuntu", ip, remoteFile)
		fmt.Println(cmdString)
	} else {
		cmdString = fmt.Sprintf("scp -i %s %s@%s:%s %s", pemLocation, "ubuntu", ip, remoteFile, filePath)
	}
	cmdWriter.Write([]byte(cmdString + "\n"))
	cmdWriter.Write([]byte("exit" + "\n"))

	err := cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}

}

func collectConfigure(ips []string, remoteFile, remoteBootstrapFile, bootStrapIP string) {

	for index, each := range ips {
		filepath := fmt.Sprintf("./storage/%d\n", index)
		fmt.Print(filepath)
		docommand(each, remoteFile, filepath, "~/Documents/thorchain_bin.pem", false)
	}
	bootStrapFilePath := fmt.Sprintf("./storage/bootstrap")
	docommand(bootStrapIP, remoteBootstrapFile, bootStrapFilePath, "~/Documents/thorchain_bin.pem", false)
}

func setupBech32Prefix() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(cmd.Bech32PrefixValAddr, cmd.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(cmd.Bech32PrefixConsAddr, cmd.Bech32PrefixConsPub)
	config.Seal()

}

func createNewConfigure(start, num int) []string {
	setupBech32Prefix()
	var newPubKeys []string
	for i := start; i < start+num; i++ {
		manager, _ := sdkkey.NewKeyManager()
		bech32key, _ := sdk.Bech32ifyAccPub(manager.GetPrivKey().PubKey())
		newPubKeys = append(newPubKeys, bech32key)
		privkeyStr, _ := manager.ExportAsPrivateKey()
		base64PrivKey := base64.StdEncoding.EncodeToString([]byte(privkeyStr))
		//fileName := fmt.Sprintf("./storage/%d", i)
		//os.Mkdir(fileName, os.ModePerm)
		input, err := ioutil.ReadFile("./storage/run_template.sh")
		if err != nil {
			panic("cannot open tempalte file")
		}
		lines := strings.Split(string(input), "\n")
		privkey := base64PrivKey
		str := fmt.Sprintf("PRIVKEY=\"%s\"", privkey)
		for i, line := range lines {
			if strings.Contains(line, "PRIVKEY") {
				lines[i] = str
				break
			}
		}
		output := strings.Join(lines, "\n")
		target := fmt.Sprintf("./storage/%d/run.sh", i)
		err = ioutil.WriteFile(target, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return newPubKeys
}

func sendRemote(ips []string, bootStrapIP, remoteFile, remoteBootstrapFile string) {
	for index, each := range ips {
		filepath := fmt.Sprintf("./storage/%d/run.sh", index)
		docommand(each, remoteFile, filepath, "~/Documents/thorchain_bin.pem", true)
	}
	bootStrapFilePath := fmt.Sprintf("./storage/0/run.sh")
	docommand(bootStrapIP, remoteBootstrapFile, bootStrapFilePath, "~/Documents/thorchain_bin.pem", true)
}

func main() {

	remoteFile := "/home/ubuntu/go-tss/benchmark_docker/Data/data_local/run.sh"
	remoteBootStrapFile := "/home/ubuntu/go-tss/benchmark_docker/Data/data_bootstrap/run.sh"

	bootStrapIP := "3.106.132.198"

	hosts, err := ioutil.ReadFile("./storage/currenthosts.txt")
	if err != nil {
		panic("cannot open tempalte file")
	}
	hostips := strings.Split(string(hosts), "\n")

	input, err := ioutil.ReadFile("./storage/testkeys.txt")
	if err != nil {
		panic("cannot open tempalte file")
	}
	testKeys := strings.Split(string(input), "\n")

	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "1":
		nodesNumStr:= os.Args[2]
		nodesNum, err := strconv.Atoi(nodesNumStr)
		if err != nil{
			fmt.Println("input error")
			return
		}
		testKeyGen(testKeys[:nodesNum],hostips[:nodesNum])
	case "2":
		nodesNumStr:= os.Args[2]
		nodesNum, err := strconv.Atoi(nodesNumStr)
		if err != nil{
			fmt.Println("input error")
			return
		}
		testKeySign(os.Args[3], hostips[:nodesNum])
	case "3":
		collectConfigure(hostips, remoteFile, remoteBootStrapFile, bootStrapIP)

	case "4":
		startPoint := 0
		numbers := 33
		newPubKeys := createNewConfigure(startPoint, numbers)
		output := strings.Join(newPubKeys, "\n")
		target := fmt.Sprintf("./storage/testkeys.txt")
		err = ioutil.WriteFile(target, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}

	case "5":
		sendRemote(hostips, bootStrapIP, remoteFile, remoteBootStrapFile)
	}

}
