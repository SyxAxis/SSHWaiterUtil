package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHScriptConfig struct {
	SSHScrCfgName           string `json:"scriptName"`
	SSHScrCfgHost           string `json:"hostname"`
	SSHScrCfgUserID         string `json:"userid"`
	SSHScrCfgPrivateKeyFile string `json:"privatekeyfile"`
	SSHScrCfgWaiterFilename string `json:"waiterFilename"`
	SSHScrCfgWaitTimeSecs   int    `json:"waitTimeSecs"`
	SSHScrCfgWaitCycles     int    `json:"waitCycles"`
}

var sshExecutionConfig SSHScriptConfig

func main() {

	sshCmdCfgFile := flag.String("scriptfile", "configTest01.json", "blah blah")
	flag.Parse()

	if InitProcess(*sshCmdCfgFile) {
		os.Exit(0)
	} else {
		os.Exit(1)
	}

}

func InitProcess(sshCmdCfgFile string) bool {

	err := ReadJSONConfigFile(sshCmdCfgFile, &sshExecutionConfig)
	if err != nil {
		log.Fatalln("Failed to read config file.")
	}

	log.Printf("%q\n", sshExecutionConfig)

	sshConn, err := OpenSSHConnection(&sshExecutionConfig)
	if err != nil {
		log.Fatalln("Connection failed.")
	}

	defer func() {
		sshConn.Close()
		log.Println("Disconnected.")
	}()

	return WaitForFile(&sshExecutionConfig, sshConn)
}

func OpenSSHConnection(sshExecutionConfig *SSHScriptConfig) (*ssh.Client, error) {
	pemBytes, err := ioutil.ReadFile(sshExecutionConfig.SSHScrCfgPrivateKeyFile)
	if err != nil {
		log.Println("Failed to read PPK file!")
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		log.Println("Failed to parse PPK data!")
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: sshExecutionConfig.SSHScrCfgUserID,
		// Auth: []ssh.AuthMethod{ ssh.Password("password"),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshConn, err := ssh.Dial("tcp", sshExecutionConfig.SSHScrCfgHost, config)
	if err != nil {
		log.Println("Failed to connect!")
		return nil, err
	}
	log.Println("Connected...")

	return sshConn, nil

}

func ReadJSONConfigFile(cfgFilename string, sshExecutionConfig *SSHScriptConfig) error {
	// Error: "invalid character 'ÿ' looking for beginning of value"
	// Issue: The text file has not been encoded with UTF8
	//        Often happens with raw Windows text files
	// Fix  : Use Powershell cmd [  cat sourcefile.json | Out-File -FilePath "targetfile.json" -Encoding "UTF8"  ]
	log.Printf("JSON cfgfile : %v\n", cfgFilename)
	rawJson, err := ioutil.ReadFile(cfgFilename)

	// JSON specs state you can simply ignore the BOM ( Byte Order Marker )
	rawJSONByte := bytes.TrimPrefix(rawJson, []byte("\xef\xbb\xbf")) // Or []byte{239, 187, 191}
	if err != nil {
		log.Println("Unable to convert JSON file content to struct.")
		return err
	}

	err = json.Unmarshal(rawJSONByte, &sshExecutionConfig)
	if err != nil {
		log.Println("Unable to exec json.Unmarshal.")
		return err
	}

	return nil
}

func ExecuteCommand(remoteCommand string, conn *ssh.Client) error {

	sess, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	err = sess.Run(remoteCommand)
	if err != nil {
		return err
	}

	return nil

}

func WaitForFile(sshExecutionConfig *SSHScriptConfig, conn *ssh.Client) bool {

	var fileFound bool = false

	lsCmd := "ls " + sshExecutionConfig.SSHScrCfgWaiterFilename
	rmCmd := "rm " + sshExecutionConfig.SSHScrCfgWaiterFilename

	loopCycle := 1
	for {
		if loopCycle > sshExecutionConfig.SSHScrCfgWaitCycles {
			break
		}

		err := ExecuteCommand(lsCmd, conn)
		if err != nil {
			log.Printf("[%v] - File not found. Waiting...\n", loopCycle)
		} else {
			err := ExecuteCommand(rmCmd, conn)
			if err != nil {
				log.Println("Failed to remove waited file.")
			}
			log.Println("File found!")
			fileFound = true
			break
		}
		time.Sleep(time.Duration(sshExecutionConfig.SSHScrCfgWaitTimeSecs) * time.Second)

		loopCycle++
	}

	return fileFound
}
