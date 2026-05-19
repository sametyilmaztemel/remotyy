# Remotty — Ares (Mac) Durum Raporu

## Versiyon Bilgisi
**Mevcut tag:** v0.7.1
**HEAD:** 34 commit ileride (Phase 1+2)
**Module:** v0.7.2-0.20260604 (pre-release)
**Aradaki fark:** 151 dosya, +18,495 / -1,523 satır

## Build Durumu

| Bileşen | Durum | Detay |
|---------|-------|-------|
| Go binary (cmd/remotty) | ✅ OK | 21MB arm64, Mach-O |
| Web UI (vite build) | ✅ OK | dist/ güncellendi (11:26) |
| Go test (16 paket) | ✅ 16/16 PASS | Tümü cached |
| Go vet | ✅ Temiz | |

## Web UI Neden Çalışmıyor?

Binary ve web build'i şu an **derlenmiş durumda** ama çalışmıyor çünkü:

1. **remotty host/signal servisi başlatılmamış** — Mac'te systemd/launchd yok, binary çalışmıyor
2. **Web UI serve edilmiyor** — `remotty signal` + `remotty host` çalışmadan web UI'ye erişilemez
3. **v0.7.1 tag'i güncellenmemiş** — Phase 1+2 kodları git'te var ama yeni release tag'i atılmamış

## v0.7.1'den Bu Yana Ne Değişti?

### Phase 1 — Foundation
- **iOS CI/CD** workflow'u eklendi
- **iOS Test Suite** — 3 dosya, ~100 XCTest
- **macOS Dağıtım Pipeline** — signing + notarization + DMG
- **iOS App Store** hazırlığı (Info.plist, ExportOptions, executable target)
- **Screen Sharing E2E** pipeline doğrulandı

### Phase 1 — Alex Tarafı (aktif deploy edildi)
- **Signal Server** → systemd, port 9010 çalışıyor
- **Host Daemon** → systemd, çalışıyor
- **File Transfer** → protocol + WebRTC data channel + web UI + host handler
- **Clipboard Sync** → protocol + web UI + Linux (wl-clip/xclip/xsel)

### Phase 2 — Hardening
- **Web Component Tests** → 50 test (17+15+18)
- **Hardening** → rate limiting, 33 config kuralı, audit log
- **Backup/Restore** → internal/backup paketi
- **TURN Server** → coturn docker-compose config'i
- **Monitoring** → Prometheus metrikleri + Grafana + health endpoints

## Eksikler

| # | Eksik | Ne Gerekli? |
|---|-------|-------------|
| 1 | **v1.0.0 release tag'i** | `git tag v1.0.0 && git push --tags` |
| 2 | **Mac'te çalıştırma** | `./remotty signal` + `./remotty host` başlat |
| 3 | **Launchd plist** | Mac'te otomatik başlama için |
| 4 | **Tauri Desktop** | Rust toolchain gerekli (`rustup`) |
| 5 | **Go test coverage** | Şu an %45, hedef %70 |
