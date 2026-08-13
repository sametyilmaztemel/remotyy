# remotty Innovation Roadmap (No-AI Track)

> Core principle: **Simple, fast, universal remote access — no AI, no complexity.**
> Macky'nin yaptığını al, daha geniş platformda, daha yetenekli, açık kaynak yap.

## remotty'in 3 Temel Farkı (Macky'ye Karşı)

| Eksen | Macky | remotty | Neden Önemli |
|-------|-------|---------|-------------|
| **Host** | Sadece macOS 15+ | macOS + Linux (ARM64/AMD64) + Windows | Oracle Cloud, Raspberry Pi, VPS, docker sunucuları |
| **Client** | Sadece iOS 18+ | Web + CLI + TUI + iOS + macOS | Her cihazdan bağlan, app gereksiz |
| **Signaling** | Velosify sunucuları ($29) | Self-hosted, sıfır maliyet | Tam kontrol, internet gerekmez, ücretsiz |

---

## Phase 1: "Better Macky" (Öncelikli — 1 Hafta)

Macky'de olanı **daha iyi** yap. Zaten çoğu tamam.

### 1.1 QR Pairing ⭐
**Etki:** Yüksek / **Çaba:** Düşük (1 gün)

Macky'de "create account → sign in on both devices → set master password" gibi bir akış var. remotty bunu **QR kod ile sıfırlıyor:**

1. Host çalışır → terminalde QR kod gösterir
2. Telefonla/browser'la QR okut → WebRTC bağlantısı kurulur
3. Master password varsa → Face ID/Touch ID ile authenticate

```
Host CLI:                    Telefon:
┌──────────────────────┐    ┌──────────────┐
│ ⎈ remotty host       │    │  📱 QR Code  │
│                      │    │              │
│ ┌──────────────────┐ │    │  Scan to     │
│ │ █▀▀▀▀▀█ █▀▀▀▀▀█ │ │───▶│  connect     │
│ │ █ ███ █ █ ███ █ │ │    │              │
│ │ █ ▀▀▀ █ █ ▀▀▀ █ │ │    │  Host:       │
│ │ ▀▀▀▀▀▀▀ ▀▀▀▀▀▀▀ │ │    │  my-server   │
│ └──────────────────┘ │    │  🔒 Encrypted │
│                      │    └──────────────┘
│ ws://192.168.1.100   │
└──────────────────────┘
```

**Implementation:** QR kod içinde `remotty://<signal-url>/<host-id>/<token>` formatı. Web client `navigator.mediaDevices.getUserMedia` ile tarar, native app AVFoundation ile.

**Macky farkı:** Macky'de hesap açman lazım. remotty'de QR bas → bağlan → bitti.

### 1.2 File Transfer (Drag-Drop)
**Etki:** Yüksek / **Çaba:** Orta (3 gün)

Zaten `internal/transfer/` paketi hazır. Eksik olan:
- **Web:** Drag-drop zone çalışıyor (FileTransfer.tsx), WebRTC data channel bağlantısı yapılacak
- **CLI:** `remotty cp <local> <host>:<path>` komutu
- **Progress:** Transfer progress bar, resume, cancel

**Macky farkı:** Macky'de file transfer **YOK**. Sadece terminal ve screen var.

### 1.3 Clipboard Sync (Bidirectional)
**Etki:** Orta / **Çaba:** Düşük (1 gün)

- Mac'te kopyala → iPhone'da yapıştır (ve tersi)
- WebRTC data channel üzerinden `clipboard` mesaj tipi
- macOS'ta `NSPasteboard` watching
- Web'de `navigator.clipboard` API
- iOS'ta `UIPasteboard`

**Macky farkı:** Macky'de clipboard sync **YOK**.

### 1.4 TURN Fallback
**Etki:** Yüksek / **Çaba:** Düşük (1 gün)

Bazı ağlarda (otel WiFi, kurumsal proxy) P2P WebRTC bağlantı kuramaz. TURN server bir relay görevi görür:
- STUN başarısız → TURN'a düş
- Latency artar ama bağlantı kopmaz
- Self-host TURN da olabilir (coturn Docker image'ı)

**Macky farkı:** Macky muhtemelen kendi TURN sunucularını kullanıyor (ücretli). remotty ile kendi TURN'unu kur + maliyet sıfır.

---

## Phase 2: "Beyond Macky" (2-4 Hafta)

Macky'de **hiç olmayan** ve bizi farklılaştıracak özellikler.

### 2.1 Port Forwarding (Killer Feature for Devs) ⭐
**Etki:** Çok Yüksek / **Çaba:** Orta (1 hafta)

WebRTC data channel üzerinden TCP tunnel. Macky'de **kesinlikle yok**.

```
# Mac'te localhost:3000'de Next.js çalışıyor
# Telefondan erişmek istiyorsun:

remotty port-forward 3000
# → https://remotty.io/tunnel/abc-123 → telefonda aç
```

Nasıl çalışır:
1. Host, localhost:3000'e TCP bağlantısı açar
2. WebRTC data channel üzerinden stream'ler
3. Client tarafında local port dinler veya web proxy gösterir
4. **Use case:** "Telefonumdan localhost'taki uygulamayı görmek istiyorum"

**Bunu yapabilen çok az tool var** (Ngrok,但要 ücretli). remotty ile bedava, kendi sunucunda, E2E encrypted.

### 2.2 Session Recording & Replay (Asciinema Style)
**Etki:** Yüksek / **Çaba:** Orta (1 hafta)

`internal/pty/recorder.go` hazır. Eksik olan:
- **Web replay:** Cast asciicast v2 formatında kaydet → web player'da oynat
- **CLI replay:** `remotty replay <session-id>`
- **Share:** Kaydı export et, başkası izlesin

**Kullanım senaryoları:**
- "Dün gece sunucuda ne yaptım?" → replay
- "Şu hatayı nasıl çözdüm?" → session'ı arkadaşına gönder
- Audit: "Kim ne zaman hangi komutu çalıştırdı?"

### 2.3 Bonjour/Local Discovery
**Etki:** Orta / **Çaba:** Düşük (2 gün)

Aynı WiFi'da host'u otomatik bul:
- Host, mDNS (Bonjour) ile `_remotty._tcp` servisi olarak yayın yapar
- Client, aynı ağdaki remotty host'larını otomatik listeler
- **Sıfır konfigürasyon:** Aynı WiFi'a bağlan → host'u gör → tıkla → bağlan

### 2.4 Multi-Session & Session Persistence
**Etki:** Yüksek / **Çaba:** Orta (4 gün)

- **Tmux entegrasyonu:** Host daemon otomatik tmux session'ı açar
  - Bağlantı kopsa bile terminal durur
  - Tekrar bağlanınca kaldığın yerden devam
- **Multi-client:** Aynı anda 2 kişi aynı terminal'i görebilir (pair programming)
- **Session list:** `remotty sessions` → aktif session'ları gör + attach

### 2.5 Wake-on-LAN + Lid-Closed Mode
**Etki:** Orta / **Çaba:** Orta (3 gün)

- **Wake-on-LAN:** Host kapalıysa → magic packet gönder → açıl → remotty bağlan
- **Lid-closed mode:** Mac kapalıyken (clamshell) çalışmaya devam et
  - macOS'ta `caffeinate` ile prevent sleep
  - Power nap ile düşük güçte bekleme

---

## Phase 3: "Enterprise & Team" (1-2 Ay)

Kurumsal kullanım için gerekli özellikler.

### 3.1 Session Audit Log
- Her bağlantı + komut kaydı
- JSON formatında export
- SIEM entegrasyonu (Syslog, Splunk, ELK)
- `audit.log` zaten hazır, export UI eklenecek

### 3.2 Time-Bound Access Tokens
```
remotty token create --duration 30m --read-only
# → tok_abc123...
# Bunu birine ver, 30dk geçerli, sadece görüntüle
```

### 3.3 Approval Workflow
- Biri bağlanmak istiyor → host'a push notification gider
- "Allow / Deny / Allow for 1 hour"
- Macky'de device approval var, ama time-based değil

### 3.4 Read-Only Mode
- Client sadece terminal çıktısını görebilir, input gönderemez
- Screen sharing sadece görüntüleme
- Support/debug senaryoları için

---

## Final Karşılaştırma: remotty vs Macky

| Özellik | Macky ($29) | remotty v0.4 | remotty v1.0 (hedef) |
|---------|------------|--------------|---------------------|
| **Host platform** | Sadece macOS | Mac + Linux ARM | Mac + Linux + Windows |
| **Client platform** | Sadece iOS | Web + CLI + TUI | Web + CLI + TUI + iOS + macOS |
| **Kurulum** | App Store + hesap | CLI + Go build | **QR ile 5 saniye** |
| **Terminal** | ✅ | ✅ | ✅ |
| **Screen** | ✅ | 🚧 | ✅ |
| **File transfer** | ❌ | 🚧 | **✅ Drag-drop** |
| **Clipboard sync** | ❌ | ❌ | **✅ Bidirectional** |
| **Port forwarding** | ❌ | ❌ | **✅ Killer feature** |
| **Session kaydı** | ❌ | 🚧 | **✅ Replay** |
| **Multi-session** | ❌ | ❌ | **✅ Tmux + pair** |
| **Bonjour discovery** | ❌ | ❌ | **✅ Zero-config** |
| **Wake-on-LAN** | ❌ | ❌ | **✅** |
| **TURN fallback** | ❌ (var mı?) | ❌ | **✅** |
| **Audit log** | ❌ | 🚧 | **✅** |
| **Time-bound token** | ❌ | ❌ | **✅** |
| **Fiyat** | $29 lifetime | **Bedava MIT** | **Bedava MIT** |
| **Signaling** | Velosify cloud | **Self-host** | **Self-host + manage** |

---

## İş Planı: Ne Zaman Ne Yapılır

```
Hafta 1-2    ████████░░░░░░░░░░░░  QR Pairing + File Transfer + Clipboard
Hafta 3-4    ██████████████░░░░░░  Port Forwarding + Bonjour Discovery
Hafta 5-6    ████████████████████  Session Recording + TURN Fallback
Hafta 7-8    ████████████████████  Multi-Session + Wake-on-LAN
Hafta 9-10   ████████████████████  Audit Log + Time-Bound Tokens
Hafta 11-12  ████████████████████  Read-Only Mode + Approval Workflow
```

---

## En Kritik 3 Özellik (MVP+)

Sana kalmış, ama bence remotty'i Macky'den **kesin olarak ayıracak** 3 şey:

1. **QR Pairing** — "App kur, hesap aç, login ol" yerine "QR oku, bağlan". Apple bile böyle yapıyor.
2. **Port Forwarding** — Developer'ların Macky yerine remotty seçmesinin #1 sebebi.
3. **Multi-platform host** — Oracle Cloud'unda çalıştır, VPS'nde çalıştır, Raspberry Pi'nde çalıştır. Macky bunların hiçbirini yapamaz (biz zaten yapıyoruz).

Bunların üçü de **AI'sız, basit, tool-level** özellikler. remotty'in prensibini bozmaz, güçlendirir.
