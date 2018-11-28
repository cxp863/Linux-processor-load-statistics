package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type statInfo struct {
	cpuCores int
	cpuAllCore [10]int64
	cpuEachCore [][10]int64
}

var chPrintStat, chCsvStat chan statInfo


func updateStatInfo(file string) {
	var thisSecondStat statInfo

	// 文件系统每次都得打开，要不得话读出来都是一样的数字，不会自动更新。。。
	inputFile, inputError := os.Open(file)
	if inputError != nil {
		fmt.Printf("An error occurred on opening the /proc/stat")
	}
	defer inputFile.Close()

	inputReader := bufio.NewReader(inputFile)
	inputString, readerError := inputReader.ReadString('\n')

	// 读不到尾巴就继续读，不判断"CPU"的原因是搞不好后面可能还得加东西
	for readerError !=io.EOF {
		if inputString[0]=='c' && inputString[1]=='p' && inputString[2]=='u' {
			var temp [10]int64
			var str string
			// 使用下划线接住不得不被接住的值，但同时不使用
			_, err := fmt.Sscanf(inputString, "%s %d %d %d %d %d %d %d", &str, &temp[0], &temp[1], &temp[2], &temp[3], &temp[4], &temp[5], &temp[6])
			if err != nil {
				fmt.Println("ERROR:", err)
			}

			// CPU后面是空格，是总的统计数据，后面跟数字，是具体核心的统计数据
			if inputString[3] == ' ' {
				thisSecondStat.cpuAllCore = temp
			} else {
				thisSecondStat.cpuEachCore = append(thisSecondStat.cpuEachCore, temp)
				thisSecondStat.cpuCores++
			}
		}

		// 读下一行
		inputString, readerError = inputReader.ReadString('\n')
	}

	chPrintStat <- thisSecondStat
	chCsvStat <- thisSecondStat

}

func printLoad() {
	// 提前读一次，避免lastSecondStat空，循环出错
	var thisSecondStat, lastSecondStat statInfo
	thisSecondStat = <- chPrintStat

	var sum int64 = 0

	for {
		lastSecondStat = thisSecondStat
		thisSecondStat = <-chPrintStat

		// 打印总CPU占用率
		sum = 0
		for i:=0; i<7; i++ {
			sum = sum + thisSecondStat.cpuAllCore[i]- lastSecondStat.cpuAllCore[i]
		}
		var idle int64 = thisSecondStat.cpuAllCore[3] - lastSecondStat.cpuAllCore[3]
		fmt.Printf("CPU: %.3f, ", 100*(1-float32(idle)/float32(sum)))

		// 打印CPU核心占用率
		for i:=0; i<thisSecondStat.cpuCores; i++ {
			sum = 0
			for j:=0; j<7; j++ {
				sum = sum + thisSecondStat.cpuEachCore[i][j]- lastSecondStat.cpuEachCore[i][j]
			}
			var idle int64 = thisSecondStat.cpuEachCore[i][3] - lastSecondStat.cpuEachCore[i][3]
			fmt.Printf("Core%d: %.3f, ",i+1 , 100*(1-float32(idle)/float32(sum)))
		}

		fmt.Print("\n")
	}
}

func saveLoad() {
	// 提前读一次，避免lastSecondStat空，循环出错
	var thisSecondStat, lastSecondStat statInfo
	thisSecondStat = <- chCsvStat

	// 创建表格标题数组
	var strarr []string = make([]string, thisSecondStat.cpuCores+1)
	for i:=0; i<len(strarr); i++ {
		if i==0 {
			strarr[i] = "CPU"
		} else {
			strarr[i] = "Core" + strconv.FormatInt(int64(i), 10)
		}
	}

	// 打开csv文件写入标题
	csvFile, err := os.Create("./statInfo.csv")
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()
	csvWriter := csv.NewWriter(csvFile)
	err = csvWriter.Write(strarr)

	// 写数据循环
	var sum int64 = 0
	for {
		lastSecondStat = thisSecondStat
		thisSecondStat = <-chCsvStat

		sum = 0
		for i:=0; i<7; i++ {
			sum = sum + thisSecondStat.cpuAllCore[i]- lastSecondStat.cpuAllCore[i]
		}
		var idle int64 = thisSecondStat.cpuAllCore[3] - lastSecondStat.cpuAllCore[3]
		strarr[0] = strconv.FormatFloat(100*(1-float64(idle)/float64(sum)), 'f', 3, 64)



		for i:=0; i<thisSecondStat.cpuCores; i++ {
			sum = 0
			for j:=0; j<7; j++ {
				sum = sum + thisSecondStat.cpuEachCore[i][j]- lastSecondStat.cpuEachCore[i][j]
			}
			var idle int64 = thisSecondStat.cpuEachCore[i][3] - lastSecondStat.cpuEachCore[i][3]
			strarr[i+1] =  strconv.FormatFloat(100*(1-float64(idle)/float64(sum)), 'f', 3, 64)
		}

		err = csvWriter.Write(strarr)
		csvWriter.Flush()
	}
}


func main() {
	// 创建两个管道
	chPrintStat = make(chan statInfo)
	chCsvStat = make(chan statInfo)

	// 启动打印和保存协程
	go printLoad()
	go saveLoad()

	// 主协程循环刷新状态，并且填到管道里，等两个协程拿走
	// 协程的for循环外边会提前拿走一次，以免开始的lastSecondStat状态是空的
	for {
		updateStatInfo(`/proc/stat`)
		time.Sleep(time.Second)
	}
}
