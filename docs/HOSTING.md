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

From source instead: `task setup`, then `task build`, then run `bin/hub`; the build puts the pinned
`livekit-server` next to it.

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
2. In the admin console, open **DNS**. Check that **MagicDNS** is on (it is for new accounts), and
   under **HTTPS Certificates** choose **Enable HTTPS**. Tailscale warns that machine names
   appear in public certificate logs; that is expected.
3. Allow Funnel in the tailnet policy; Funnel stays off for every machine until you do. In the
   admin console, open **Access controls → JSON editor**. Just after the first `{`, add:

   ```json
   "nodeAttrs": [
     { "target": ["autogroup:member"], "attr": ["funnel"] },
   ],
   ```

   If the policy already has a `"nodeAttrs": [` line, add only the middle line inside that list.
   Leave the existing access rules as they are and **Save**; the editor refuses to save a policy
   with a mistake in it.
4. In the hub, choose **Tailscale Funnel**, keep the machine name (`dislackso`) or pick another,
   and **Save and restart**.
5. The status shows **Sign in to Tailscale**. Open it, sign in with the account from step 1 and
   approve the machine. The status then says it is getting a certificate, then that it waits for
   the address to work from the internet: the first time, Tailscale takes up to 10 minutes to
   publish it. It turns green once the hub has opened the address from outside itself.

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

### Public address with a free DuckDNS name

When the router gives this machine a public IP (DMZ or port 443 forwarded to it), the hub can serve
the internet itself, with no tunnel in between. DuckDNS gives a free `<name>.duckdns.org` and the
hub keeps it pointed at your public IP, also after the provider changes it.

1. Sign in at [duckdns.org](https://www.duckdns.org) (GitHub or Google), add a subdomain and copy
   the **token** at the top of the page.
2. In the hub, choose **Public address with a domain**, set **Keep the domain pointed here** to
   **DuckDNS**, enter `<name>.duckdns.org`, paste the token, leave the listen address at `:443`
   and **Save and restart**.
3. When Windows asks whether the hub may use the network, tick **Private** and **Public** and allow
   it. Let's Encrypt connects to port 443 to issue the certificate.
4. The status goes through "Pointing the domain…" and "Getting an HTTPS certificate…" to
   reachable, usually within a minute. The address becomes the instance's public URL.

On the hub machine itself keep using `http://localhost:7300`: some routers do not loop the public
address back to the inside.

With DuckDNS the certificate comes over Let's Encrypt's DNS check: the hub publishes a TXT record
through DuckDNS, so Let's Encrypt never connects to this machine. This matters on home connections
whose provider drops connections from abroad, where port 443 works for your friends but not for
Let's Encrypt. Each request first runs on Let's Encrypt's staging server, so a setup that cannot
pass never uses up the real allowance; the first certificate takes a few minutes.

**Your own domain, kept pointed by DuckDNS.** To use a subdomain of a domain you own (say
`chat.example.com`) and still follow a changing public IP, add two **CNAME** records in your
domain's DNS, both to `<name>.duckdns.org`: one named `chat` and one named `_acme-challenge.chat`.
In the hub, enter `chat.example.com` as the domain, keep DuckDNS, and put `<name>.duckdns.org` in
**DuckDNS name**. The hub keeps the DuckDNS name pointed here and answers the DNS check through it.

If a certificate check fails, the status shows Let's Encrypt's reason (usually that port 443 does
not reach this machine) and the hub waits 15 minutes before the next check, since Let's Encrypt
allows a name only 5 failed checks an hour. **Save and restart** checks again at once.

### Other choices

- **Public address with your own domain**: the machine has a public IP, your domain's DNS points
  at it, and port 443 reaches it. The hub gets a Let's Encrypt certificate itself.
- **This network only**: devices on your network open `https://<this machine's IP>:7443`. The
  certificate is self-signed, so each browser warns once.
- **Only this machine**: the default; nobody else can reach the hub.

## 4. Check calls and add a relay

The **Voice reachability** test on the Connectivity page (also run at every start) rates how
people outside your network connect to calls:

| Rating | Meaning | What to do |
|---|---|---|
| Excellent | The router forwards the media ports, the machine has a public address, or it has IPv6 | Nothing. A relay still helps friends whose own network is strict. |
| Good | The router keeps one outside port per inside port, so most calls connect directly | Turn on UPnP (see below) or add a relay for strict networks. |
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

## 7. The desktop app (optional)

Everyone can use Dislackso in Chrome or Edge. The desktop app adds one thing for now: whoever
shares their screen from it sees the drawings on their real monitor.

1. Open the project's latest release on GitHub and download the installer: `Dislackso-Setup-….exe`
   on Windows, the `.dmg` on macOS (`arm64` for Apple chips), the `.AppImage` or `.deb` on Linux.
2. Windows: run it. SmartScreen says it protected your PC: click **More info** → **Run anyway**. It
   installs for you only and opens. macOS: open the dmg, drag Dislackso to Applications, then
   right-click it → **Open** the first time.
3. Paste the hub's address (the one you open in the browser) and press **Connect**, then sign in.

Closing the window keeps the app (and a call) running in the tray, next to the clock; right-click
its icon → **Quit Dislackso** to close it for good.

The app updates itself from new releases (on macOS it shows where to download the new one). To
check right away: Settings → **Desktop app** → **Check for updates**, or right-click the tray icon →
**Check for updates**. Most
changes need no new release at all: the app shows the web app the hub serves, so updating the hub
updates everyone.

## Troubleshooting

| Status or symptom | Fix |
|---|---|
| "Turn on HTTPS certificates for your tailnet" | Tailscale admin console → DNS → HTTPS Certificates → Enable. |
| "Your tailnet does not allow Funnel" | Add the `funnel` node attribute in Access controls → JSON editor (step 3 of the Tailscale path); wait a minute. |
| "Could not download cloudflared" | The hub retries; check that GitHub is reachable from this machine. |
| "Could not point the domain at this machine" | Check the DuckDNS token and that the name is one you added on duckdns.org. |
| "Let's Encrypt is not issuing a certificate for this name right now" | Too many failed tries recently. Leave the hub running; it tries again at the time in the message. |
| "Let's Encrypt could not verify the domain" | Its reason is under the message. Without DuckDNS, "Timeout during connect" or "Connection refused" means port 443 does not reach this machine from abroad (router forward, Windows Firewall, or a provider that filters it); DuckDNS avoids that check. |
| "The record Let's Encrypt checks does not show in public DNS" | With your own domain, add the CNAME from `_acme-challenge.<subdomain>` to the DuckDNS name. |
| "Could not get a certificate" | Port 443 must reach this machine: router DMZ or forward, and Windows Firewall allowing the hub. Test port 443 with an online port checker while the hub runs. |
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
