package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"
)

type TextStats struct {
	Grams       map[string]int64
	UniqueRunes map[rune]struct{}
}

func NewTextStats() TextStats {
	result := TextStats{
		Grams:       make(map[string]int64),
		UniqueRunes: make(map[rune]struct{}),
	}
	return result
}

func (s *TextStats) MergeFrom(other TextStats) error {
	for key, value := range other.Grams {
		s.Grams[key] += value
	}
	for runeValue := range other.UniqueRunes {
		s.UniqueRunes[runeValue] = struct{}{}
	}
	return nil
}

func ParseFile(filename string, gramSize int, out chan<- TextStats) {
	stats := NewTextStats()
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	runesBuffer := make([]rune, gramSize)
	for idx := 0; idx < gramSize; idx += 1 {
		runesBuffer[idx] = rune(' ')
	}
	offset := 0
	fileEnded := false
	runesRead := 0
	for {
		runeValue := unicode.ReplacementChar
		for runeValue == unicode.ReplacementChar {
			var err error
			runeValue, _, err = r.ReadRune()
			if err == io.EOF {
				fileEnded = true
				break
			}
			if err != nil {
				log.Fatal(err)
			}
		}
		if fileEnded {
			break
		}
		runesRead += 1
		stats.UniqueRunes[runeValue] = struct{}{}
		offset = (offset + 1) % gramSize
		runesBuffer[offset] = runeValue
		gram := ""
		for idx := 0; idx < gramSize; idx += 1 {
			runeValue := runesBuffer[(gramSize+offset-idx)%gramSize]
			gram = string(runeValue) + gram
			stats.Grams[gram] += 1
		}
	}
	log.Printf("File %v read %v runes.\n", filename, runesRead)
	out <- stats
}

func Predict(textPart string, stats *TextStats, alphabet []rune) rune {
	var sum int64 = 0
	weights := make([]int64, len(alphabet))
	for idx := 0; idx < len(alphabet); idx += 1 {
		weights[idx] = stats.Grams[textPart+string(alphabet[idx])]
		sum += weights[idx]
	}
	if sum < int64(len(alphabet)) {
		return Predict(textPart[1:], stats, alphabet)
	}
	chosenIndex := rand.Int63n(sum)
	for idx := 0; idx < len(alphabet); idx += 1 {
		if chosenIndex < weights[idx] {
			return alphabet[idx]
		}
		chosenIndex -= weights[idx]
	}
	panic("Error while sampling symbol")
}

func main() {
	var textsDir string
	for {
		fmt.Print("Provide folder with text files:")
		_, err := fmt.Scanln(&textsDir)
		if err == nil && textsDir != "" {
			if _, err := os.Stat(textsDir); err == nil {
				break
			}
		}
	}

	files, err := os.ReadDir(textsDir)
	if err != nil {
		panic(err)
	}

	gramSize := 8
	workersCount := 8
	jobs := make(chan string, len(files))
	results := make(chan TextStats, workersCount)
	for i := 0; i < workersCount; i += 1 {
		go func(input <-chan string, output chan<- TextStats) {
			for filename := range input {
				ParseFile(filename, gramSize, output)
			}
		}(jobs, results)
	}
	for _, entity := range files {
		if entity.IsDir() {
			continue
		}
		jobs <- filepath.Join(textsDir, entity.Name())
	}
	close(jobs)
	totalStats := NewTextStats()
	for i := 0; i < len(files); i += 1 {
		stats := <-results
		totalStats.MergeFrom(stats)
	}
	fmt.Printf("'это' stats: %v\n", totalStats.Grams["это"])
	fmt.Printf("'кес' stats: %v\n", totalStats.Grams["кес"])
	fmt.Printf("'\\n' stats: %v\n", totalStats.Grams["\n"])
	var max int64 = 0
	popKey := ""
	for key, count := range totalStats.Grams {
		if count < 0 {
			panic("non-positive count detected")
		}
		if len(key) < gramSize {
			continue
		}
		if count > max {
			max = count
			popKey = key
		}
	}
	fmt.Printf("Best key '%v' stats: %v\n\n", popKey, max)

	for i := 0; i < 5; i += 1 {
		textLen := 500
		text := ""
		alphabet := make([]rune, len(totalStats.UniqueRunes))
		idx := 0
		for key := range totalStats.UniqueRunes {
			alphabet[idx] = key
			idx += 1
		}
		for i := 0; i < textLen; i += 1 {
			part := ""
			if utf8.RuneCountInString(text) >= gramSize {
				width := 0
				for i := 0; i < gramSize-1; i += 1 {
					_, size := utf8.DecodeLastRuneInString(text[:len(text)-width])
					width += size
				}
				part = text[len(text)-width:]
			} else {
				part = text
			}
			newRune := Predict(part, &totalStats, alphabet)
			text += string(newRune)
		}
		fmt.Println(text)
	}
}
