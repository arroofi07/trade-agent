# White Paper Pengguna
# AI Trading Insight API

## 1. Ringkasan

`AI Trading Insight API` adalah layanan analisis pasar crypto yang membantu pengguna mendapatkan insight trading lebih cepat dan terstruktur.  
Sistem ini menggabungkan data market realtime, indikator teknikal, analisis kuantitatif, dan validasi AI untuk menghasilkan sinyal:

- `LONG` (bias naik),
- `SHORT` (bias turun),
- `WAIT` (menunggu, belum ada setup yang baik).

Tujuan utama produk ini bukan menjanjikan profit instan, melainkan membantu pengambilan keputusan yang lebih disiplin, terukur, dan berbasis data.

## 2. Masalah yang Diselesaikan

Banyak trader ritel menghadapi tantangan:

- Overload informasi dari banyak chart dan indikator.
- Sulit menilai apakah sinyal benar-benar kuat atau hanya noise.
- Keputusan emosional karena tidak ada kerangka evaluasi yang konsisten.
- Tidak punya ringkasan cepat untuk entry, stop loss, dan target profit.

Platform ini hadir untuk merangkum data kompleks menjadi output yang lebih mudah dipahami dan dapat ditindaklanjuti.

## 3. Untuk Siapa Produk Ini

Produk ini cocok untuk:

- Trader crypto pemula yang ingin insight terstruktur.
- Trader intermediate yang ingin mempercepat proses screening pair.
- Developer/automation builder yang ingin mengintegrasikan sinyal ke bot/monitoring.

Produk ini **tidak** ditujukan sebagai pengganti edukasi trading dasar dan manajemen risiko.

## 4. Cara Kerja (Versi Sederhana)

Sistem bekerja dalam beberapa tahap:

1. Mengambil data harga dan volume pasar secara realtime.
2. Menghitung indikator teknikal dari data candlestick.
3. Membaca pola pergerakan harga (pattern recognition).
4. Menghasilkan skor kuantitatif untuk menilai kekuatan setup.
5. Menjalankan validasi AI agar reasoning lebih kontekstual.
6. Menggabungkan seluruh hasil menjadi satu keputusan akhir.

Hasil akhir dikirim dalam format JSON agar mudah dibaca manusia maupun dipakai aplikasi lain.

## 5. Nilai Utama untuk User

- **Lebih cepat**: dari data mentah menjadi insight siap pakai.
- **Lebih konsisten**: keputusan berbasis kerangka analitik, bukan feeling sesaat.
- **Lebih terukur**: ada confidence score dan level risiko.
- **Lebih fleksibel**: bisa dipakai manual atau diintegrasikan ke sistem otomatis.

## 6. Fitur Utama yang Paling Berguna

- `Price`  
  Cek harga, perubahan 24 jam, dan volume.
- `Trending`  
  Melihat pair yang paling aktif berdasarkan volume.
- `Analyze (v1)`  
  Analisis teknikal + AI dasar (kompatibilitas lama).
- `Analyze V2`  
  Analisis lebih lengkap: indikator + pattern + quant + multi-timeframe + AI validation.
- `History`  
  Menyimpan jejak hasil analisis sebelumnya untuk evaluasi.

## 7. Cara Baca Hasil Analisis

Output penting yang perlu dipahami user:

- `final_signal` / `signal`  
  Arahan utama: `LONG`, `SHORT`, atau `WAIT`.
- `confidence`  
  Tingkat keyakinan model (0-100).  
  Semakin tinggi, semakin kuat konfirmasi data (bukan jaminan profit).
- `reasoning`  
  Penjelasan kenapa sinyal itu muncul.
- `entry`, `stop_loss`, `tp1`, `tp2`, `tp3`  
  Rekomendasi area masuk, batas kerugian, dan target bertahap.
- `risk_reward`  
  Rasio potensi profit dibanding potensi risiko.

## 8. Contoh Alur Penggunaan Harian

Contoh workflow user:

1. Cek pair aktif di endpoint `trending`.
2. Pilih 2-5 pair terbaik.
3. Panggil `analyze-v2` untuk masing-masing pair.
4. Prioritaskan pair dengan:
   - signal jelas (`LONG`/`SHORT`),
   - confidence memadai,
   - risk-reward sehat,
   - reasoning yang konsisten.
5. Tetap validasi dengan plan risiko pribadi sebelum eksekusi.

## 9. Prinsip Manajemen Risiko (Wajib)

Gunakan sistem ini secara bertanggung jawab:

- Risiko per posisi disarankan kecil (mis. 1-2% modal).
- Selalu gunakan stop loss.
- Hindari overtrade hanya karena banyak sinyal.
- Jangan membuka posisi saat sinyal `WAIT` jika strategi kamu butuh konfirmasi kuat.
- Review performa berkala dari data history.

## 10. Batasan Produk

Penting dipahami:

- Model analisis bisa salah saat market ekstrem/berita besar.
- Latency jaringan/API pihak ketiga dapat mempengaruhi freshness data.
- Confidence tinggi bukan jaminan hasil pasti.
- Insight AI dipengaruhi kualitas data market yang tersedia.

Karena itu, output sistem sebaiknya dipakai sebagai **decision support**, bukan autopilot tanpa kontrol.

## 11. Keamanan dan Privasi

Prinsip umum layanan:

- Kunci API disimpan sebagai environment variable di server.
- Data analisis historis disimpan di database untuk evaluasi performa.
- Praktik keamanan tetap perlu dijaga oleh operator (rotasi key, pembatasan akses, monitoring).

## 12. FAQ Singkat

**Q: Apakah sistem ini menjamin profit?**  
A: Tidak. Sistem ini meningkatkan kualitas keputusan, bukan menjamin hasil trading.

**Q: Kapan sebaiknya mengikuti sinyal `WAIT`?**  
A: `WAIT` berarti market belum memberi setup jelas. Biasanya lebih aman menunggu konfirmasi.

**Q: Kenapa sinyal AI dan quant bisa berbeda?**  
A: Keduanya melihat market dari sudut yang berbeda. Sistem menggabungkan keduanya agar keputusan lebih stabil.

**Q: Bisa dipakai untuk bot trading otomatis?**  
A: Bisa. API dirancang agar output JSON mudah diintegrasikan ke bot/automation pipeline.

## 13. Disclaimer

Dokumen dan layanan ini disediakan untuk tujuan edukasi dan decision support.  
Trading aset kripto memiliki risiko tinggi, termasuk potensi kehilangan modal secara signifikan.  
Setiap keputusan transaksi sepenuhnya menjadi tanggung jawab pengguna.

---

Jika dibutuhkan, dokumen ini bisa dikembangkan menjadi versi:

- **End User Guide** (lebih praktis, langkah demi langkah),
- **API Integration Guide** (khusus developer),
- **Risk Policy Handbook** (khusus tim/internal trading desk).
