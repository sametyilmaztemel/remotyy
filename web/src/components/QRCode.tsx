interface Props {
  data: string;
  size?: number;
}

export default function QRCode({ data, size = 200 }: Props) {
  // Use a public QR code API to generate the QR image
  const apiUrl = `https://api.qrserver.com/v1/create-qr-code/?size=${size}x${size}&data=${encodeURIComponent(data)}`;

  return (
    <div style={{ textAlign: 'center' }}>
      <img
        src={apiUrl}
        alt="QR Code"
        width={size}
        height={size}
        style={{ imageRendering: 'pixelated', borderRadius: 8 }}
        onError={(e) => {
          // Fallback: render a simple text-based representation
          const target = e.target as HTMLImageElement;
          target.style.display = 'none';
          const parent = target.parentElement;
          if (parent) {
            const fallback = document.createElement('div');
            fallback.style.cssText = 'padding: 12px;font-size:10px;word-break:break-all;color:#666;';
            fallback.textContent = `remotty://${data.slice(0, 60)}...`;
            parent.appendChild(fallback);
          }
        }}
      />
      <p style={{ fontSize: 10, color: 'var(--text-dim)', marginTop: 4 }}>
        Scan with camera to connect
      </p>
    </div>
  );
}
