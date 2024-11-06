package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var validDomains = []string{}
var showAllLogs = false // Variável de controle para exibir todos os logs
var userProxy1 = false
var userProxy2 = false
var ipexterno = ""

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func logMessage(message string) {
	if showAllLogs {
		fmt.Println(message)
	}
}

func getMXRecords(domain string) ([]string, error) {
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return nil, fmt.Errorf("could not get MX records: %v", err)
	}

	var servers []string
	for _, mx := range mxRecords {
		servers = append(servers, mx.Host)
	}
	return servers, nil
}

func dialWithHTTPProxy(proxyURL, username, password, mxServer, port string) (net.Conn, error) {
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse proxy URL: %v", err)
	}

	conn, err := net.Dial("tcp", proxyURLParsed.Host)
	if err != nil {
		return nil, fmt.Errorf("could not connect to proxy: %v", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	connectReq := fmt.Sprintf("CONNECT %s:%s HTTP/1.1\r\nHost: %s:%s\r\nProxy-Authorization: Basic %s\r\n\r\n", mxServer, port, mxServer, port, auth)
	_, err = conn.Write([]byte(connectReq))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not write connect request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not read connect response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("connect request failed: %v", resp.Status)
	}

	return conn, nil
}
func testSMTPConnection(mxServer string, port string) error {
	var conn net.Conn
	var err error

	dialDirectly := func() (net.Conn, error) {
		conn, err := net.DialTimeout("tcp", mxServer+":"+port, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("could not connect to SMTP server %s on port %s: %v", mxServer, port, err)
		}
		return conn, nil
	}

	if userProxy1 {
		conn, err = dialWithHTTPProxy("http://"+os.Getenv("PROXY_HOST")+":"+os.Getenv("PROXY_PORT"), os.Getenv("PROXY_USERNAME")+"-zone-resi-region-br", os.Getenv("PROXY_PASSWORD"), mxServer, port)
	} else {
		conn, err = dialDirectly()
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	resp, err := readSMTPResponse(reader)
	if err != nil {
		return fmt.Errorf("could not read initial response from %s on port %s: %v", mxServer, port, err)
	}
	logMessage(fmt.Sprintf("Initial response from %s on port %s: %s", mxServer, port, resp))

	return nil
}
func readSMTPResponse(reader *bufio.Reader) (string, error) {
	var resp strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		resp.WriteString(line)
		if len(line) < 4 || line[3] != '-' {
			break
		}
	}
	return resp.String(), nil
}

func verifyEmail(email, mxServer, port, fromdomain, from string) (int, error) {
	var conn net.Conn
	var err error

	dialDirectly := func() (net.Conn, error) {
		conn, err := net.DialTimeout("tcp", mxServer+":"+port, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("could not connect to SMTP server %s on port %s: %v", mxServer, port, err)
		}
		return conn, nil
	}

	if userProxy2 {
		conn, err = dialWithHTTPProxy("http://"+os.Getenv("PROXY_HOST")+":"+os.Getenv("PROXY_PORT"), os.Getenv("PROXY_USERNAME")+"-zone-resi-region-br", os.Getenv("PROXY_PASSWORD"), mxServer, port)
	} else {
		conn, err = dialDirectly()
	}
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	resp, err := readSMTPResponse(reader)
	if err != nil {
		return 0, fmt.Errorf("could not read initial response: %v", err)
	}
	logMessage(fmt.Sprintf("Initial response: %s", resp))

	fmt.Fprintf(conn, "EHLO "+fromdomain+"\r\n")
	resp, err = readSMTPResponse(reader)
	if err != nil {
		return 0, fmt.Errorf("could not read EHLO response: %v", err)
	}
	logMessage(fmt.Sprintf("EHLO response: %s", resp))

	fmt.Fprintf(conn, "MAIL FROM:<"+from+">\r\n")
	resp, err = readSMTPResponse(reader)
	if err != nil {
		return 0, fmt.Errorf("could not read MAIL FROM response: %v", err)
	}
	logMessage(fmt.Sprintf("MAIL FROM response: %s", resp))

	fmt.Fprintf(conn, "RCPT TO:<%s>\r\n", email)
	resp, err = readSMTPResponse(reader)
	if err != nil {
		return 0, fmt.Errorf("could not read RCPT TO response: %v", err)
	}
	logMessage(fmt.Sprintf("RCPT TO response: %s", resp))

	if strings.HasPrefix(resp, "250") {
		return 1, nil
	} else if strings.HasPrefix(resp, "550") {
		//verifica se o email está na blacklist buscando strings com nomes de blacklist
		if strings.Contains(resp, "spfbl") || strings.Contains(resp, "banned") || strings.Contains(resp, "spamhaus") ||
			strings.Contains(resp, "blocked") || strings.Contains(resp, "dnsbl") || strings.Contains(resp, "spamrats") ||
			strings.Contains(resp, "barracuda") || strings.Contains(resp, "invaluement") || strings.Contains(resp, "junke") ||
			strings.Contains(resp, "redhawk") || strings.Contains(resp, "sorbs") || strings.Contains(resp, "spamcop") || strings.Contains(resp, ipexterno) {
			return 3, nil
		} else {
			return 2, nil
		}
	} else if strings.HasPrefix(resp, "421 4.7.1") {
		return 3, nil
	} else {
		return 0, fmt.Errorf("unexpected response: %s", resp)
	}
}

func getRandomMailFrom() (string, string) {
	firstNames := []string{
		"ricardo", "lucia", "carlos", "mariana", "joao", "ana", "pedro", "beatriz", "paulo", "fernanda",
		"maria", "jose", "antonio", "roberta", "rafael", "juliana", "gabriel", "camila", "mateus", "leticia",
		"rodrigo", "patricia", "bruno", "isabela", "gustavo", "tania", "marcelo", "raquel", "diego", "bruna",
		"andre", "alice", "felipe", "valeria", "victor", "natalia", "leandro", "renata", "murilo", "aline",
		"igor", "elaine", "vinicius", "karla", "samuel", "luana", "henrique", "simone", "roberto", "luiza",
		"thiago", "vanessa", "otavio", "sara", "mario", "rosana", "daniel", "regina", "lucas", "carla",
		"davi", "angela", "edson", "priscila", "mateus", "helena", "fernando", "adriana", "tiago", "renato",
		"matheus", "sofia", "jorge", "diana", "vitor", "cristina", "hugo", "tatiana", "alexandre", "lara",
		"nicolas", "stefany", "murilo", "michele", "vinicius", "paula", "leonardo", "alessandra", "raul", "viviane",
		"arthur", "milena", "gustavo", "bianca", "miguel", "jaqueline", "caio", "tamires", "heitor", "brenda",
	}

	lastNames := []string{
		"silva", "pereira", "santos", "oliveira", "souza", "lima", "ferreira", "costa", "rodrigues", "martins",
		"almeida", "araujo", "ribeiro", "mendes", "barbosa", "moura", "pires", "teixeira", "fernandes", "gomes",
		"machado", "dias", "freitas", "ramos", "rezende", "nunes", "carvalho", "tavares", "peixoto", "melo",
		"miranda", "campos", "santiago", "vieira", "cardoso", "castro", "pires", "andrade", "dantas", "monteiro",
		"bernardo", "assis", "viana", "morais", "alves", "siqueira", "faria", "matos", "barros", "azevedo",
		"baptista", "moreira", "lopes", "sousa", "pinheiro", "cruz", "rocha", "albuquerque", "correia", "duarte",
		"garcia", "sousa",
	}

	rand.Seed(time.Now().UnixNano())
	domain := validDomains[rand.Intn(len(validDomains))]
	nome := firstNames[rand.Intn(len(firstNames))]
	sobrenome := lastNames[rand.Intn(len(lastNames))]
	return domain, fmt.Sprintf("%s.%s@%s", nome, sobrenome, domain)
}

func Check(email string) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		logMessage("Invalid email format")
		return
	}
	domain := parts[1]
	//verifica ser é ig.com.br e não usa proxy
	if domain == "ig.com.br" && !userProxy2 {
		fmt.Printf("%s - IG\n", email)
		err := saveEmails("IG.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
		return
	} else if domain == "terra.com.br" && !userProxy2 {
		fmt.Printf("%s - TERRA\n", email)
		err := saveEmails("Terra.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
		return
	}

	mxServers, err := getMXRecords(domain)
	if err != nil {
		fmt.Printf("Error getting MX records: %v\n", err)
		return
	}
	//adiciona possíveis servidores de email
	mxServers = append(mxServers, "mail."+domain)
	mxServers = append(mxServers, "smtp."+domain)

	ports := []string{"25"}
	var workingServer string
	var workingPort string

	for _, mxServer := range mxServers {
		for _, port := range ports {
			logMessage(fmt.Sprintf("Testing connection to %s on port %s...\n", mxServer, port))
			err := testSMTPConnection(mxServer, port)
			if err != nil {
				logMessage(fmt.Sprintf("Error: %v\n", err))
			} else {
				logMessage(fmt.Sprintf("Successfully connected to %s on port %s\n", mxServer, port))
				workingServer = mxServer
				workingPort = port
				break
			}
		}
		if workingServer != "" {
			break
		}
	}

	if workingServer == "" {
		fmt.Printf("%s - UNKNOWN\n", email)
		err := saveEmails("Unknow.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
		return
	} else {
		msg := fmt.Sprintf("%s - %s", email, workingServer)
		palavrasproibidas := []string{"barracuda", "spam", "abuse", "blacklist", "block", "dnsbl", "spamhaus", "sorbs", "spamrats", "invaluement", "redhawk", "junke", "banned", "blocked", "spfbl", "spamcop", "backlist"}

		for _, palavra := range palavrasproibidas {
			if strings.Contains(msg, palavra) {
				err := saveEmails("Servidores_proibidos.txt", []string{msg})
				if err != nil {
					fmt.Printf("Error saving email: %v\n", err)
				}
				return
			}
		}

		//salva emails que não são proibidos
		err := saveEmails("Emails_validos.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
		//tirar esse return depois
		return
		// verifica sem workingServer contem .outlook.com
		if strings.Contains(workingServer, ".outlook.com") {
			err := saveEmails("Outlook.txt", []string{email})
			if err != nil {
				fmt.Printf("Error saving email: %v\n", err)
			}
			return
		}

		err = saveEmails("Servidores.txt", []string{msg})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}

	}

	logMessage(fmt.Sprintf("Using server %s on port %s to verify email...\n", workingServer, workingPort))
	fromdomain, from := getRandomMailFrom()
	logMessage(fmt.Sprintf("From address: %s\n", from))
	valid, err := verifyEmail(email, workingServer, workingPort, fromdomain, from)
	if err != nil {
		fmt.Printf("%s - ERROR - %s\n", email, workingServer)
		err := saveEmails("Error.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
		return
	}
	if valid == 1 {
		fmt.Printf("%s - LIVE - %s\n", email, workingServer)
		err := saveEmails("LIVE.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
	} else if valid == 2 {
		fmt.Printf("%s - DIE - %s\n", email, workingServer)
		err := saveEmails("DIE.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
	} else if valid == 3 {
		fmt.Printf("%s - BLACKLIST - %s\n", email, workingServer)
		err := saveEmails("Backlistip.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
	} else {
		fmt.Printf("%s - DIE - %s\n", email, workingServer)
		err := saveEmails("DIE.txt", []string{email})
		if err != nil {
			fmt.Printf("Error saving email: %v\n", err)
		}
	}
}

func saveEmails(filename string, emails []string) error {
	file, err := os.OpenFile("./retornos/"+filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, email := range emails {
		_, err := writer.WriteString(email + "\n")
		if err != nil {
			return fmt.Errorf("could not write to file: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("could not flush writer: %v", err)
	}

	return nil
}

func main() {

	//pega o valor da variável de ambiente
	userProxy1, _ = strconv.ParseBool(os.Getenv("USE_PROXY1"))
	userProxy2, _ = strconv.ParseBool(os.Getenv("USE_PROXY2"))
	thread, _ := strconv.Atoi(os.Getenv("THREADS"))
	showAllLogs, _ = strconv.ParseBool(os.Getenv("LOGVIEWER"))
	ipexterno = os.Getenv("IP_EXTERNO")

	//verifica se tem a pasta retornos, se não tiver cria
	if _, err := os.Stat("retornos"); os.IsNotExist(err) {
		os.Mkdir("retornos", 0755)
	}

	//alimenta validDomains com o valid_send_domains.txt
	file, err := os.Open("valid_send_domains.txt")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		validDomains = append(validDomains, strings.TrimSpace(scanner.Text()))
	}
	emails := readEmails()

	var wg sync.WaitGroup
	sem := make(chan struct{}, thread) // Limita o número de goroutines

	for _, email := range emails {

		wg.Add(1)
		sem <- struct{}{}
		go func(email string) {
			defer wg.Done()
			Check(email)
			<-sem
		}(email)

	}
	wg.Wait()
}

func readEmails() []string {
	file, err := os.Open("emails.txt")
	if err != nil {
		log.Println("Erro ao abrir o arquivo contas.txt:", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var servers []string
	for scanner.Scan() {
		servers = append(servers, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Println("Erro ao ler o arquivo contas.txt:", err)
		return nil
	}
	return servers
}
