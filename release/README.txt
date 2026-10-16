Dislackso hub
=============

Start "hub" (on Windows, double-click hub.exe). On the first run your browser
opens the setup page; if it does not, open the address the hub prints.

Keep livekit-server next to the hub: it carries voice and screen shares.

After setup, Settings -> Connectivity makes the hub reachable for your team.
The full guide, in English, is docs/HOSTING.md in the source repository.

Commands:
  hub            start the hub
  hub doctor     check how well this machine can host calls
  hub help       list commands and options

Data (database, uploads, logs) lives in your user's application data folder:
  Windows  %AppData%\Dislackso\hub
  macOS    ~/Library/Application Support/Dislackso/hub
  Linux    ~/.local/share/dislackso/hub
