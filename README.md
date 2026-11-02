### Internet Shutdown: How we will read the news

### One Picture

![Internet Shutdown: How we will read the news](picture.svg)

> [!WARNING]
Unfortunately, the image above represents the target vision for the final project.<br>
This repository contains just a simple proof-of-concept without overcomplication, ensuring the example runs for 99% of testers at `1 kbit/s.`

### In a nutshell
We want to show that it is impossible to completely block access to information in the modern world.<br>
The main idea of the project is to prove that even if an international channel is choked down to just `a few kbit/s`, it is still entirely realistic to read news, use Telegram, and chat with AI right inside your terminal. This method remains viable as long as citizens on the inside and the outside world retain access to any national or corporate services. In this scenario, custom tunneling solutions on GitHub can carry your terminal's bytes across the border.

### Did you mean SSH?
Basically, yes. But we want to separate `TIO` (Terminal Interaction Object) from SSH, because the latter has become too deeply rooted in people's minds as the *only* possible terminal communication interface (`TIO`) between machines. It might surprise you, but a terminal session doesn't have to rely on SSH. SSH is merely one, albeit the most common, implementation.

### Why not just use SSH?
In principle, regular SSH can also work; it only requires about `8 kbit/s`. However, it relies on TCP and a persistent connection.<br>
We want solutions to appear on GitHub that can maintain a terminal even when there is no persistent connection as such, and withstand network interruptions of any duration.

### Okay, but how can you read news, Telegram, and chat with AI on a few kbit/s?
Of course, this is impossible within the traditional paradigms of the modern internet.
However, we can change the approach. Raw textual information only takes up bytes. Everything else is just wrapper bloat.

### Can you elaborate?
For example, Telegram channels can be represented as:
```bash
# This translates to reading the last 10 posts in the weather forecast channel
tg-cli read-channel @weather -p 10
```

Chatting with an AI can look like this:
```bash
chatgpt-cli ask What is the weather today?
```

More advanced interfaces, such as ANSI sequences and terminal pseudo-graphics, are also possible.<br>
It seems it's time to remember those technologies.

### Who on earth would read news through a terminal?
You might not need this right now, but it's worth preparing for scenarios like those seen in Iran.

### How it works

> [!NOTE]
> The `notssh` client sits on a near-dead channel, while the `notssh-server` is hosted abroad, where you can host CLI versions of all the best tools.<br>
> The bridge between them is implemented using custom tunneling solutions found on GitHub.

```bash
+---------------+           +------------------+                             +------------------+           +---------------+
|               |    IPC    |                  |     Custom async-bridge     |                  |    IPC    |               |
| notssh-client | <=======> |   Custom Bridge  | <========================>  |  Custom Bridge   | <=======> | notssh-server |
|  (1 kbit/s)   | [len+pld] |      Client      |    (Email, Chat, Video)     |      Server      | [len+pld] |    (abroad)   |
|               |           |                  |                             |                  |           |               |
+---------------+           +------------------+                             +------------------+           +---------------+
```

> [!NOTE]
> At the core of the notssh server lies the idea of a PTY. Simply put, the computer believes that the notssh-server program is a real keyboard and screen.

### Why this separation?
- 1) `notssh` is the core engine. Its purpose is to provide `TIO` (Terminal Interaction Object) between machines while delivering a user experience similar to SSH.<br>
However, the core itself doesn't know how or through what channel the traffic will flow, and it cannot account for transport-specific characteristics.<br>
It simply takes in and outputs bytes via UDP sockets on `localhost`.<br>
This is the simplest possible implementation of IPC (Inter-Process Communication) between programs.

- 2) `1kbit-transport` is a reference transport implementation. In this specific case, it's a UDP relay that paces bytes to enforce a strict `1 kbit/s` limit. Its goal is to demonstrate that `TIO` doesn't have to rely on SSH!

- 3) Essentially, the core supports any `*-transport` project. Today it's a pedagogical 1kbit implementation, tomorrow it could be a tunnel over email, chat, video, and so on.

### How to run

> [!NOTE]
> This guide assumes that all machines are running Linux.<br>
> Other operating systems have not been tested.

1. Open two console windows on both the client and the server.

2. Clone both projects on each machine:

```bash
git clone https://github.com/linuxdev345/notssh
```
```bash
git clone https://github.com/linuxdev345/1kbit-transport
```

3. Edit `1kbit-transport` on both the client and the server. Open `config.toml` and update the parameters to match your configuration.
4. Edit `notssh` on both the client and the server. Open `config.toml` and update the parameters to match your configuration.

5. Build the client core:
```bash
cd notssh
rm -r server
cd client
go mod init client
go mod tidy
go build -o notssh-client .
```

6. Build the server core:
```bash
cd notssh
rm -r client
cd server
go mod init server
go mod tidy
go build -o notssh-server .
```

7. Build the transport:
```bash
cd 1kbit-transport
go mod init 1kbit-transport
go mod tidy
go build -o 1kbit-transport .
```

> [!NOTE]
> These utilities do not take any command-line arguments.

8. Run `1kbit-transport` in the first tab **on both the client and the server**:
```bash
cd 1kbit-transport
chmod +x ./1kbit-transport
./1kbit-transport
```

9. Run `notssh-client` in the second tab **on the client**:
```bash
cd notssh/client
chmod +x ./notssh-client
./notssh-client
```

10. Run `notssh-server` in the second tab **on the server:**
```bash
cd notssh/server
chmod +x ./notssh-server
./notssh-server
```

> [!TIP]
> After `Transport Connected` appears, press `Enter` to wake up the console.

### A few words about pull requests
This project was created as a proof-of-concept (PoC) with no plans for ongoing maintenance.<br>
Feel free to fork and evolve your ideas in your own repositories!

> ⚠️ **Disclaimer**<br>
> The `notssh` project is distributed "AS IS" — without any warranties of stability, security, or fault tolerance.<br>
> This is a demonstration protocol, not intended for use in critical infrastructure or production environments.<br>
> **Use at your own risk.**
