# AI Trading API (Crypto Analysis)

Proyek ini adalah backend API berbasis Go untuk analisis pasar crypto secara otomatis menggunakan kombinasi:

- data market realtime (utama: Indodax),
- indikator teknikal,
- pattern recognition,
- scoring kuantitatif,
- validasi AI (Claude/GPT) dengan fallback.

Tujuan utamanya adalah menghasilkan sinyal trading yang lebih robust (`LONG`, `SHORT`, `WAIT`) beserta confidence, reasoning, dan level manajemen risiko (entry, stop loss, take profit).

## Gambaran Singkat Arsitektur

Project ini menggunakan arsitektur layer sederhana dan mudah di-scale:

- `main.go`  
  Entry point aplikasi, load config, koneksi database, setup Fiber + middleware.
- `config/`  
  Membaca environment variable dari `.env`.
- `routes/`  
  Registrasi endpoint API dan dependency injection service ke handler.
- `handlers/`  
  HTTP handler untuk endpoint crypto (v1 & v2).
- `services/`  
  Seluruh business logic:
  - market provider (`IndodaxService`, dll),
  - indikator teknikal,
  - pattern analysis,
  - quantitative scoring,
  - AI provider orchestration (Claude primary / GPT fallback atau sebaliknya).
- `database/` + `models/`  
  Koneksi Supabase PostgreSQL + schema data history analisis.
- `migrations/`  
  SQL migrasi pelengkap untuk riwayat analisis.

## Alur Analisis (V2)

Untuk endpoint `analyze-v2`, alurnya secara high-level:

1. Ambil ticker realtime + OHLCV (candles).
2. Hitung indikator teknikal.
3. Deteksi pola harga/candlestick.
4. Hitung skor kuantitatif dari gabungan indikator + pattern.
5. Jalankan multi-timeframe analysis (15m, 1h, 4h) untuk confluence.
6. Kirim context ke AI provider:
   - primary sesuai `AI_MODEL`,
   - fallback provider jika primary gagal.
7. Gabungkan hasil quant + AI menjadi sinyal final.
8. Simpan riwayat analisis ke database (async).

Pendekatan ini membuat sistem tetap bisa memberi output walau AI provider sedang unavailable (graceful degradation ke quant signal).

## Fitur Utama

- Endpoint harga realtime (`price`)
- Endpoint candlestick (`klines`)
- Endpoint indikator teknikal (`indicators`)
- Endpoint trending market (`trending`)
- Analisis AI v1 (backward compatible)
- Analisis AI v2 (quant + pattern + MTF + AI validation)
- Penyimpanan history analisis ke Supabase PostgreSQL
- Fallback AI provider (Claude <-> GPT)

## Endpoint Penting

Base URL default: `http://localhost:3000`

- `GET /health`
- `GET /api/crypto/price/:symbol`
- `GET /api/crypto/klines/:symbol?timeframe=1h&limit=100`
- `GET /api/crypto/indicators/:symbol?timeframe=1h`
- `GET /api/crypto/trending?limit=20`
- `GET /api/crypto/analyze/:symbol?timeframe=1h` (v1)
- `GET /api/crypto/analyze-v2/:symbol?timeframe=1h` (v2)
- `GET /api/crypto/history/:symbol?limit=20`

Contoh symbol yang umum dipakai di project ini adalah pasangan berbasis IDR (mis. `BTCIDR`, `ETHIDR`), dengan normalisasi symbol yang fleksibel.

## Konfigurasi Environment

Buat file `.env` di root project dan isi minimal:

```env
PORT=3000

# Database (wajib)
DATABASE_URL=postgres://USER:PASSWORD@HOST:PORT/DBNAME?sslmode=require
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_KEY=your_supabase_key

# AI (pilih model utama)
AI_MODEL=claude
ANTHROPIC_API_KEY=your_anthropic_key
OPENAI_API_KEY=your_openai_key
OPENAI_MODEL=gpt-4o

# Optional tuning untuk Indodax service
INDODAX_HTTP_TIMEOUT_SEC=45
# INDODAX_USDT_IDR=16500
```

Catatan penting:

- `DATABASE_URL` wajib terisi, jika tidak aplikasi akan stop saat boot.
- Jika `AI_MODEL=claude`, maka idealnya `ANTHROPIC_API_KEY` tersedia.
- Jika `AI_MODEL=gpt`, maka idealnya `OPENAI_API_KEY` tersedia.
- Saat primary AI gagal, service akan mencoba fallback provider jika key fallback tersedia.

## Cara Menjalankan

### 1) Install dependency

```bash
go mod tidy
```

Atau via Makefile:

```bash
make install
```

### 2) Jalankan server

```bash
make run
```

Untuk mode development (hot reload):

```bash
make dev
```

Server akan aktif di:

`http://localhost:3000`

## Saran Pengembangan Lanjutan

- Tambahkan autentikasi + rate limiting untuk endpoint publik.
- Tambahkan test otomatis (unit test service + integration test handler).
- Tambahkan observability (structured logging, metrics, tracing).
- Pisahkan strategi sinyal ke modul policy terpisah agar lebih mudah tuning.
- Dokumentasikan kontrak response API dalam format OpenAPI/Swagger.

---

Jika kamu mau, saya bisa lanjutkan step berikutnya: buatkan `API_REFERENCE.md` khusus dokumentasi request/response JSON per endpoint supaya siap dipakai frontend atau integrasi bot.
