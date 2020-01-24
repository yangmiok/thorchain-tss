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
	"gitlab.com/thorchain/thornode/cmd"
)

const baseTssPort = 8080


// KeySignResp key sign response
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

type AwsIps[][]struct {
	Instance string `json:"Instance"`
	PubIP    string `json:"PubIp"`
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

func docommand(ip, remoteFile, filePath, pemLocation string, send bool, username string, digitalocean bool) {
	var out bytes.Buffer
	cmd := exec.Command("bash")
	cmdWriter, _ := cmd.StdinPipe()
	cmd.Start()
	cmd.Stdout = &out
	cmdString := ""
	if send {
		if digitalocean{
			cmdString = fmt.Sprintf("scp  %s %s@%s:%s",filePath, username, ip, remoteFile)
		}else{

			cmdString = fmt.Sprintf("scp -i %s %s %s@%s:%s", pemLocation, filePath, username, ip, remoteFile)
		}
		fmt.Println(cmdString)
	} else {
		if digitalocean{
			cmdString = fmt.Sprintf("scp  %s@%s:%s %s", username, ip, remoteFile, filePath)
		}else{
			cmdString = fmt.Sprintf("scp -i %s %s@%s:%s %s", pemLocation, username, ip, remoteFile, filePath)
		}
	}
	cmdWriter.Write([]byte(cmdString + "\n"))
	cmdWriter.Write([]byte("exit" + "\n"))

	err := cmd.Wait()
	if err != nil {
		fmt.Println(err)
	}

}

func collectConfigure(ips []string, remoteFile, remoteBootstrapFile, bootStrapIP string, username string, isdigitalOcean bool) {

	for index, each := range ips {
		filepath := fmt.Sprintf("benchmark_client/storage/%d\n", index)
		fmt.Print(filepath)
		docommand(each, remoteFile, filepath, "~/Documents/thorchain_bin.pem", false, username, isdigitalOcean)
	}
	bootStrapFilePath := fmt.Sprintf("benchmark_client/storage/bootstrap")
	docommand(bootStrapIP, remoteBootstrapFile, bootStrapFilePath, "~/Documents/thorchain_bin.pem", false,username, isdigitalOcean)
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
		input, err := ioutil.ReadFile("benchmark_client/storage/run_template.sh")
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
		target := fmt.Sprintf("benchmark_client/storage/%d/run.sh", i)
		err = ioutil.WriteFile(target, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return newPubKeys
}

func sendRemote(ips []string, bootStrapIP, remoteFile, remoteBootstrapFile string, username string, isDigitalOcean bool) {
	for index, each := range ips {
		filepath := fmt.Sprintf("benchmark_client/storage/%d/run.sh", index+1)
		docommand(each, remoteFile, filepath, "~/Documents/thorchain_bin.pem", true, username, isDigitalOcean)
	}
	bootStrapFilePath := fmt.Sprintf("benchmark_client/storage/0/run.sh")
	docommand(bootStrapIP, remoteBootstrapFile, bootStrapFilePath, "~/Documents/thorchain_bin.pem", true, username, isDigitalOcean)
}

func generatehostIPs(path string,outfile string){
	jsonFile, err := os.Open(path)
	if err != nil{
		fmt.Println("error in open the IP json file")
	}
	defer jsonFile.Close()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil{
		fmt.Println("error in load the IP json file")
	}

	var awsIps AwsIps
	var ips []string
	json.Unmarshal(byteValue, &awsIps)
	for _,eachRegion := range awsIps{
		for _,instance := range eachRegion{
			ips = append(ips, instance.PubIP)
		}
	}
	writefd, err:= os.Create(outfile)
	if err != nil{
		fmt.Println("error in open the file to write")
		return
	}
	defer writefd.Close()
	for _, each := range ips{
		fmt.Println(each)
		_, err :=writefd.WriteString(each+"\n")
		if err != nil{
			fmt.Println(err)
		}
	}
}


func main() {

	remoteFile := "/home/ubuntu/go-tss/benchmark_docker/Data/data_local/run.sh"
	remoteBootStrapFile := "/home/ubuntu/go-tss/benchmark_docker/Data/data_bootstrap/run.sh"

	bootStrapIP := "167.71.204.54"

	hosts, err := ioutil.ReadFile("benchmark_client/storage/currenthosts.txt")
	if err != nil {
		panic("cannot open template file")
	}
	hostips := strings.Split(string(hosts), "\n")

	input, err := ioutil.ReadFile("benchmark_client/storage/testkeys.txt")
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
		collectConfigure(hostips, remoteFile, remoteBootStrapFile, bootStrapIP, "root", true)

	case "4":
		// create new environment, it will overwrite current keypairs!!!
		startPoint := 0
		numbers := 33
		newPubKeys := createNewConfigure(startPoint, numbers)
		output := strings.Join(newPubKeys, "\n")
		target := fmt.Sprintf("benchmark_client/storage/testkeys.txt")
		err = ioutil.WriteFile(target, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	case "5":
		// send the keypairs to remote and configure the remote machines
		sendRemote(hostips, bootStrapIP, remoteFile, remoteBootStrapFile, "root", true)
	case "6":
		// run command
		//aws ec2 --region ap-southeast-1 \
		//describe-instances \
		//--filter "Name=key-name, Values=thorchain_bin"\
		//--query 'Reservations[*].Instances[*].{Instance:InstanceId,Subnet:PublicIpAddress}' \
		//--output json
		// to generate the IP address of all the instance firstly and copy the json file to rawip.txt
		generatehostIPs("benchmark_client/storage/ips.json", "benchmark_client/storage/currenthosts.txt")
	}
}
