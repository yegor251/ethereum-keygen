package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
)

var (
	words []string
)

func main() {
	f, err := os.Create("memory_profile.pprof")
	if err != nil {
		log.Fatalf("Ошибка при создании файла профиля памяти: %v", err)
	}
	defer f.Close()

	err = pprof.WriteHeapProfile(f)
	if err != nil {
		log.Fatalf("Ошибка при записи профиля памяти: %v", err)
	}

	words, err = readSeedFromFile("start.txt")
	if err != nil {
		log.Fatalf("Ошибка при считывании seed-фраз из файла: %v", err)
	}

	client, err := ethclient.Dial("http://127.0.0.1:8545")
	if err != nil {
		log.Fatalf("Ошибка подключения: %v", err)
	}
	defer client.Close()

	var found bool
	var combinationsChecked int
	var wg sync.WaitGroup
	sem := make(chan struct{}, 2)

	generateSeedPermutations(words, func(seed string) {
		sem <- struct{}{}
		wg.Add(1)

		go func(seed string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			if checkWallet(client, seed, combinationsChecked) {
				found = true
				writeSeedToFile(seed)
			}
			combinationsChecked++
			if combinationsChecked%100000 == 0 {
				err := writeSeedToFileWithReset(words)
				if err != nil {
					log.Printf("Ошибка при перезаписи файла: %v", err)
				}
			}
		}(seed)
	})

	wg.Wait()

	if !found {
		fmt.Println("Все комбинации проверены. Баланс везде пуст.")
	} else {
		fmt.Println("Ненулевой баланс найден!")
	}

	fmt.Printf("Проверено %d комбинаций.\n", combinationsChecked)
	fmt.Println("Программа завершена.")
}

func generateSeedPermutations(words []string, process func(string)) {
	helper(words, 0, process)
}

func helper(arr []string, k int, process func(string)) {
	if k == len(arr) {
		process(strings.Join(arr, " "))
		return
	}
	for i := k; i < len(arr); i++ {
		arr[k], arr[i] = arr[i], arr[k]
		helper(arr, k+1, process)
		arr[k], arr[i] = arr[i], arr[k]
	}
}

func checkWallet(client *ethclient.Client, seed string, num int) bool {
	privateKey, err := generatePrivateKeyFromSeed(seed)
	if err != nil {
		log.Printf("Ошибка генерации приватного ключа: %v", err)
		return false
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		log.Printf("Ошибка получения баланса: %v, %s\n", err, seed)
		return false
	}

	if balance.Cmp(big.NewInt(0)) > 0 {
		fmt.Printf("Найден ненулевой баланс! Баланс: %s, Seed: %s\n", balance.String(), seed)
		return true
	} else {
		fmt.Printf("Баланс: %s, Seed: %s, номер:%d\n", balance.String(), seed, num)
	}

	return false
}

func generatePrivateKeyFromSeed(seed string) (*ecdsa.PrivateKey, error) {
	hash := crypto.Keccak256Hash([]byte(seed)).Bytes()
	return crypto.ToECDSA(hash)
}

func writeSeedToFile(seed string) {
	file, err := os.OpenFile("found_seeds.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Ошибка при открытии файла: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(seed + "\n")
	if err != nil {
		log.Fatalf("Ошибка при записи в файл: %v", err)
	}
	fmt.Println("Seed фраза записана в файл:", seed)
}

func readSeedFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var seeds []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		words := strings.Fields(line)
		seeds = append(seeds, words...)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	fmt.Printf("Считанные seed фразы: %v\n", seeds)
	return seeds, nil
}

func writeSeedToFileWithReset(seeds []string) error {
	file, err := os.Create("start.txt")
	if err != nil {
		return err
	}
	defer file.Close()

	for _, seed := range seeds {
		_, err := file.WriteString(seed + "\n")
		if err != nil {
			return err
		}
	}

	fmt.Println("Файл start.txt перезаписан.")
	return nil
}
