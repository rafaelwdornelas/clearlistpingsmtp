
# Verificação de Email com Proxy - Email Verification with Proxy

## 📋 Sobre o Projeto | About the Project

**Português:**  
Este projeto é um verificador de emails desenvolvido em Go que se conecta a servidores SMTP para verificar se um email é válido. Ele suporta a utilização de proxies HTTP e autenticação básica, e pode ser configurado para logar as respostas completas do servidor SMTP. Além disso, identifica se o email está listado em blacklists conhecidas, utilizando filtros para respostas de servidores.

**English:**  
This project is an email verifier developed in Go that connects to SMTP servers to check if an email is valid. It supports HTTP proxies and basic authentication and can be configured to log full SMTP server responses. Additionally, it detects if the email is listed on known blacklists by filtering server responses.

## ⚙️ Funcionalidades | Features

- **Verificação de Email | Email Verification**: Verifica a existência de um email via servidores SMTP.
- **Suporte a Proxy | Proxy Support**: Conecta-se via proxy HTTP com autenticação básica.
- **Log Personalizável | Customizable Logging**: Controla a quantidade de logs exibidos, ideal para depuração detalhada.
- **Detecção de Blacklists | Blacklist Detection**: Identifica se o email está em blacklists conhecidas por meio de filtros de resposta.

## 🚀 Começando | Getting Started

### Pré-requisitos | Prerequisites

- Go 1.19+
- Arquivo `.env` com as configurações de proxy, conforme o modelo abaixo.

```plaintext
# .env
PROXY_HOST=proxy.exemplo.com
PROXY_PORT=8080
PROXY_USERNAME=seu_usuario
PROXY_PASSWORD=sua_senha
```

### Instalação | Installation

1. Clone o repositório:
   ```bash
   git clone https://github.com/rafaelwdornelas/clearlistpingsmtp.git
   cd seu_repositorio
   ```

2. Instale as dependências necessárias:
   ```bash
   go mod tidy
   ```

3. Configure o arquivo `.env` com as variáveis de ambiente de proxy.

### Uso | Usage

Execute o programa passando o email e domínio que deseja verificar:

```bash
go run main.go
```

### Exemplo de Código | Code Example

```go
// Exemplo de função para testar a conexão SMTP
func testSMTPConnection(mxServer string, port string) error {
    var conn net.Conn
    var err error

    if userProxy1 {
        conn, err = dialWithHTTPProxy("http://"+os.Getenv("PROXY_HOST")+":"+os.Getenv("PROXY_PORT"), os.Getenv("PROXY_USERNAME")+"-zone-resi-region-br", os.Getenv("PROXY_PASSWORD"), mxServer, port)
    } else {
        conn, err = net.Dial("tcp", mxServer+":"+port)
    }

    if err != nil {
        return fmt.Errorf("Erro ao conectar: %v", err)
    }
    defer conn.Close()
    
    fmt.Fprintf(conn, "EHLO domain.com\r\n")
    // Resto da verificação...
}
```

## 📄 Licença | License

Este projeto está licenciado sob a Licença MIT. Consulte o arquivo `LICENSE` para mais detalhes.

## 📞 Suporte | Support

Se precisar de ajuda, sinta-se à vontade para abrir uma [issue](https://github.com/rafaelwdornelas/clearlistpingsmtp/issues) no repositório.

---
