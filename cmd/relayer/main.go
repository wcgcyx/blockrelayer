package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "blockrelayer",
		Usage: "A simple block relayer",
		Commands: []*cli.Command{
			{
				Name: "start",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "writer_ap",
						Value: "http://127.0.0.1:8545",
						Usage: "specify writer ap",
					},
					&cli.StringFlag{
						Name:  "reader_ap",
						Value: "http://127.0.0.1:8645",
						Usage: "specify reader ap",
					},
					&cli.Uint64Flag{
						Name:  "target",
						Value: 0,
						Usage: "specify sync target",
					},
					&cli.Uint64Flag{
						Name:  "single_block",
						Value: 0,
						Usage: "specify single block to import",
					},
				},
				Action: func(c *cli.Context) error {
					return start(c.String("writer_ap"), c.String("reader_ap"), c.Uint64("target"), c.Uint64("single_block"))
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func start(writerAPI, readerAPI string, target uint64, single uint64) error {
	if single != 0 {
		block, err := getTrustedByNumber(writerAPI, single)
		if err != nil {
			return err
		}
		err = importTrustedBlock(readerAPI, block)
		if err != nil {
			return err
		}
		fmt.Printf("Imported single block #%v\n", single)
		return nil
	}
	// Make sure reader side is always 256 away from writer side
	readerNumber, err := getCurrentBlockNumber(readerAPI)
	if err != nil {
		return err
	}
	writerNumber, err := getCurrentBlockNumber(writerAPI)
	if err != nil {
		return err
	}
	if target != 0 {
		for target > readerNumber {
			block, err := getTrustedByNumber(writerAPI, readerNumber+1)
			if err != nil {
				return err
			}
			err = importTrustedBlock(readerAPI, block)
			if err != nil {
				return err
			}
			readerNumber++
			fmt.Printf("Imported #%v, Target #%v\n", readerNumber, target)
		}
		fmt.Println("Target reached.")
		return nil
	}
	for {
		for writerNumber-readerNumber > 256 {
			block, err := getTrustedByNumber(writerAPI, readerNumber+1)
			if err != nil {
				return err
			}
			err = importTrustedBlock(readerAPI, block)
			if err != nil {
				return err
			}
			readerNumber++
			fmt.Printf("Imported #%v, Target #%v\n", readerNumber, writerNumber-256)
		}
		fmt.Println("Waiting for new target...")
		writerNumber, err = getCurrentBlockNumber(writerAPI)
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
}

func getCurrentBlockNumber(api string) (uint64, error) {
	client, err := ethclient.Dial(api)
	if err != nil {
		return 0, err
	}
	defer client.Close()
	num, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, err
	}
	return num, nil
}

func getTrustedByNumber(writerAPI string, num uint64) (string, error) {
	var jsonStr = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"admin_retrieveTrustedBlock","params":[false, false, %v],"id":1}`, num))
	req, err := http.NewRequest("POST", writerAPI, bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 || strings.Contains(string(body), "error") {
		return "", fmt.Errorf(string(body))
	}
	return strings.Split(strings.Split(string(body), `"result" : "`)[1], "\"")[0], nil
}

func getTrustedByHash(writerAPI string, hash string) (string, error) {
	var jsonStr = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"admin_retrieveTrustedBlock","params":[false, true, "%v"],"id":1}`, hash))
	req, err := http.NewRequest("POST", writerAPI, bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 || strings.Contains(string(body), "error") {
		return "", fmt.Errorf(string(body))
	}
	return strings.Split(strings.Split(string(body), `"result" : "`)[1], "\"")[0], nil
}

func importTrustedBlock(readerAPI string, block string) error {
	var jsonStr = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"admin_importTrustedBlock","params":["%v"],"id":1}`, block))
	req, err := http.NewRequest("POST", readerAPI, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || strings.Contains(string(body), "error") {
		return fmt.Errorf(string(body))
	}
	return nil
}
