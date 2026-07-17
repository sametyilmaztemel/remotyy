import { useState, useRef, useCallback, useEffect } from 'react';

interface QRScannerProps {
  onScan: (url: string) => void;
  onClose: () => void;
}

export default function QRScanner({ onScan, onClose }: QRScannerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState('');
  const [active, setActive] = useState(false);
  const streamRef = useRef<MediaStream | null>(null);
  const scanTimerRef = useRef<number>(0);

  const startCamera = useCallback(async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: 'environment', width: 480, height: 480 }
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        videoRef.current.play();
        setActive(true);
      }
    } catch (err) {
      setError('Camera access denied. Allow camera permission and try again.');
    }
  }, []);

  const stopCamera = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach(t => t.stop());
      streamRef.current = null;
    }
    if (scanTimerRef.current) {
      window.clearInterval(scanTimerRef.current);
    }
  }, []);

  useEffect(() => {
    startCamera();
    return () => stopCamera();
  }, [startCamera, stopCamera]);

  const scanQR = useCallback(async () => {
    if (!videoRef.current || !canvasRef.current) return;

    const canvas = canvasRef.current;
    const video = videoRef.current;
    canvas.width = video.videoWidth || 320;
    canvas.height = video.videoHeight || 320;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);

    // Try native BarcodeDetector first
    try {
      const detector = new (window as any).BarcodeDetector({ formats: ['qr_code'] });
      const barcodes = await detector.detect(canvas);
      for (const barcode of barcodes) {
        const url = barcode.rawValue;
        if (url && (url.startsWith('remotty://') || url.startsWith('ws://') || url.startsWith('wss://'))) {
          stopCamera();
          onScan(url);
          return;
        }
      }
    } catch {}

    // Fallback: keep scanning
    scanTimerRef.current = window.setTimeout(() => scanQR(), 500);
  }, [onScan, stopCamera]);

  useEffect(() => {
    if (active) {
      scanTimerRef.current = window.setTimeout(() => scanQR(), 500);
    }
    return () => {
      if (scanTimerRef.current) window.clearTimeout(scanTimerRef.current);
    };
  }, [active, scanQR]);

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      background: 'rgba(0,0,0,0.9)', zIndex: 1000,
      display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
      padding: 20,
    }}>
      <div style={{
        width: 300, height: 300, borderRadius: 16, overflow: 'hidden',
        border: '2px solid var(--accent)', position: 'relative',
      }}>
        <video ref={videoRef} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
        <canvas ref={canvasRef} style={{ display: 'none' }} />
        <div style={{
          position: 'absolute', top: '50%', left: '50%',
          width: 200, height: 200,
          transform: 'translate(-50%, -50%)',
          border: '2px dashed rgba(139,92,246,0.4)',
          borderRadius: 12,
          pointerEvents: 'none',
        }} />
      </div>

      <p style={{ color: '#fff', margin: '16px 0 8px', fontSize: 14 }}>
        Point camera at a remotty QR code
      </p>
      {error && <p style={{ color: '#ef4444', fontSize: 12, marginBottom: 8 }}>{error}</p>}

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          onClick={() => { stopCamera(); onClose(); }}
          style={{
            padding: '8px 24px', background: 'var(--accent)', color: '#fff',
            border: 'none', borderRadius: 6, cursor: 'pointer', fontSize: 13,
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
