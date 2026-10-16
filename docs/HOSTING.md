# Hosting a Dislackso instance

One person on the team runs the **hub** on their own computer. Everyone else opens the hub's
address in a browser. You need no server, no port forwarding and no paid service.

This guide goes in order. Steps 1–3 are enough for a first call with people on other networks;
the rest make it reliable.

## 1. Run the hub

1. Download the archive for your system from the project's Releases page:
   `dislackso-hub-<version>-windows-amd64.zip` for most Windows PCs, `darwin-arm64` for Apple
   Silicon Macs, `linux-amd64` for most Linux PCs. `checksums.txt` lists each archive's SHA-256.
2. Extract it to a folder of its own and keep `hub` and `livekit-server` together.
3. Start `hub` (on Windows, double-click `hub.exe`). A console window shows the hub's log; keep
   it open while you host. Closing it stops the hub. The binaries are not signed yet:
   - Windows SmartScreen may say "Windows protected your PC": choose **More info → Run anyway**.
     Windows Firewall may ask about `livekit-server`: allow it on private networks.
   - macOS blocks downloaded binaries: run `xattr -d com.apple.quarantine hub livekit-server` in
     the folder once, then `./hub`.
4. Your browser opens the setup page. If it does not, open the `http://localhost:…/setup?token=…`
   address printed in the console.

From source instead: `task setup`, then `task build`, then run `bin/hub` with `livekit-server`
from `.tools/bin` next to it (or pass `--livekit-binary`).

## 2. Set up the instance

The wizard asks for the instance name, your admin account and, optionally, email (step 5 below
explains Gmail; you can skip it now). When you finish, the **Connectivity** page opens.

## 3. Make the hub reachable

On **Settings → Connectivity → Public address**, choose how people reach the hub, then **Save
and restart**. The status above the form says when the address works, and the address becomes
the instance's public URL (invite links and emails use it).

### Tailscale Funnel (recommended)

A free, stable address like `https://dislackso.your-tailnet.ts.net`. Nothing to install: the hub
carries its own Tailscale node.

1. Create a free account at <https://login.tailscale.com/start> (Google or GitHub sign-in works).
2. In the admin console, open **DNS** and, under **HTTPS Certificates**, choose **Enable HTTPS**.
3. In the hub, choose **Tailscale Funnel**, keep the machine name (`dislackso`) or pick another,
   and **Save and restart**.
4. The status shows **Sign in to Tailscale**. Open it, sign in with the account from step 1 and
   approve the machine. Within a few seconds the status turns green with the address.
5. If the status says Funnel is not allowed: in the admin console, open **Access controls** and
   allow Funnel for your machines (the console offers to add the `funnel` attribute). The hub
   retries on its own; wait a minute.

The sign-in is kept in the hub's data folder, so restarts need no new login.

### Cloudflare quick tunnel (first try, no account)

Choose **Cloudflare quick tunnel**. The hub downloads `cloudflared` (pinned version, checksum
verified) and shows a random `https://….trycloudflare.com` address. It changes every time the hub
starts, so invite links stop working after a restart; use it to try things out.

### Cloudflare Tunnel (your own domain)

For a stable address on a domain you manage on Cloudflare (the free plan is enough):

1. In the Cloudflare dashboard, open **Zero Trust → Networks → Tunnels → Create a tunnel**,
   choose **Cloudflared**, and name it.
2. On the install page, copy the long token after `--token` in any of the commands. You do not
   need to install anything.
3. Add a **Public hostname**: pick a subdomain and your domain (for example `chat.example.com`);
   service **HTTP**, URL `localhost:7301`.
4. In the hub, choose **Cloudflare Tunnel (own domain)**, enter `https://chat.example.com` and the
   token, and **Save and restart**.

### Other choices

- **Public address with a domain**: the machine has a public IP, your domain's DNS points at it,
  and port 443 reaches it. The hub gets a Let's Encrypt certificate itself.
- **This network only**: devices on your network open `https://<this machine's IP>:7443`. The
  certificate is self-signed, so each browser warns once.
- **Only this machine**: the default; nobody else can reach the hub.

## 4. Check calls and add a relay

The **Voice reachability** test on the Connectivity page (also run at every start) rates how
people outside your network connect to calls:

| Rating | Meaning | What to do |
|---|---|---|
| Excellent | The router forwards the media ports, the machine has a public address, or it has IPv6 | Nothing. A relay still helps friends whose own network is strict. |
| Good | The router keeps ports, so most calls connect directly | Turn on UPnP (see below) or add a relay. |
| Needs a relay | The router changes ports per destination (strict NAT, CGNAT) | Add a relay (below). |
| Offline | No internet at start | Connect and restart the hub. |

**Port forwarding.** The hub asks the router to forward UDP 7882 and TCP 7881 over PCP, NAT-PMP
or UPnP and keeps the forwarding up. If the test says it is not available, turn on UPnP (or
NAT-PMP) in your router's settings, or forward those two ports to this machine by hand, and run
the test again. `hub doctor` runs the same checks from a console.

### Cloudflare relay (TURN), free for a small team

A relay carries calls only for people who cannot connect directly. Cloudflare gives 1,000 GB a
month free.

1. In the Cloudflare dashboard, open **Realtime → TURN Server → Create**, and name the key.
2. Copy the **Turn Token ID** and the **API Token** it shows.
3. In the hub, **Voice relay (TURN)**: choose **Cloudflare Realtime TURN**, paste both, keep the
   monthly budget (900 GB) and **Save**.
4. **Test the relay**: your browser connects through the relay alone. "The relay works" means
   people on any network can join calls.

The page shows this month's relayed traffic and where the month is heading. Past the budget the
hub stops handing out relay access until the next month, unless you tick "Keep the relay on past
the budget".

## 5. Email with Gmail (optional)

With email, new accounts confirm their address and people reset forgotten passwords themselves.

1. Turn on **2-Step Verification** for the Google account at
   <https://myaccount.google.com/security>.
2. Create an **app password** at <https://myaccount.google.com/apppasswords> (any name, such as
   "Dislackso"). Google shows 16 letters once.
3. In the hub, **Settings → Instance → Email**: tick "Send email through an SMTP server", server
   `smtp.gmail.com`, port `587`, security **STARTTLS**, username your Gmail address, password the
   app password, From address your Gmail address. **Save**, then **Send test email**.

Gmail sends up to about 500 emails a day, far more than a team needs.

## 6. Hosting conveniences

On the Connectivity page, from the hosting machine itself:

- **Keep this computer awake while the hub runs** (on by default): the screen may turn off, the
  computer does not sleep.
- **Start the hub when I sign in**: starts it the same way you started it this time.
- **Allow voice through Windows Firewall**: if calls from outside fail on Windows, this adds an
  inbound rule for `livekit-server.exe`; Windows asks you to confirm.

## Troubleshooting

| Status or symptom | Fix |
|---|---|
| "Turn on HTTPS certificates for your tailnet" | Tailscale admin console → DNS → HTTPS Certificates → Enable. |
| "Your tailnet does not allow Funnel" | Admin console → Access controls → allow Funnel; wait a minute. |
| "Could not download cloudflared" | The hub retries; check that GitHub is reachable from this machine. |
| "Another program uses that port" | Something else listens on the address (443, 7443 or 7301); stop it or choose another. |
| Calls connect at home but not for friends | Run the reachability test; add a relay; allow the firewall rule. |
| The hub window closes at once | Start it from a terminal (`hub`) to read the error; on Windows the window now waits for Enter after an error. |
| Voice "not available" | `livekit-server` must be next to the hub; `logs/livekit.log` in the data folder says why it stopped. |

Data folder: Windows `%AppData%\Dislackso\hub`, macOS `~/Library/Application Support/Dislackso/hub`,
Linux `~/.local/share/dislackso/hub`. It holds the database, uploads, `secrets.yaml` (keys and
passwords; keep it private) and `logs/`.

## Ports

| Port | Where | What |
|---|---|---|
| 7300/tcp | 127.0.0.1 only | The hub (web app, API, gateway) |
| 7301/tcp | 127.0.0.1 only | Where a Cloudflare named tunnel delivers traffic |
| 7880/tcp | 127.0.0.1 only | The media server's API and signaling (through the hub at `/lk`) |
| 7882/udp | all interfaces | Call media; forwarded on the router when possible |
| 7881/tcp | all interfaces | Call media over TCP when UDP is blocked |
| 443/tcp, 7443/tcp | when chosen | "Public address" and "This network only" |

## Bandwidth

Every viewer receives each stream from the host, so the host's upload is the limit:

| Scenario | Host upload (peak) | Host download |
|---|---|---|
| Voice only, 10 people | ~5 Mbps | ~1 Mbps |
| 6 people, one 1080p60 share | ~32 Mbps | ~8 Mbps |
| 6 people, two 1440p60 shares | ~60 Mbps | ~25 Mbps |

Text and code shares use far less than these peaks while the screen is still.
