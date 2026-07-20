import { createContext, useContext, useEffect, useState, useRef, type ReactNode } from 'react';
import { SignalingClient } from '../lib/signaling';

interface SignalingContextType {
  client: SignalingClient | null;
  status: string;
  connect: () => Promise<void>;
  disconnect: () => void;
}

const SignalingContext = createContext<SignalingContextType>({
  client: null,
  status: 'disconnected',
  connect: async () => {},
  disconnect: () => {},
});

export function SignalingProvider({ url, children }: { url: string; children: ReactNode }) {
  const clientRef = useRef<SignalingClient | null>(null);
  const [status, setStatus] = useState('disconnected');

  useEffect(() => {
    const client = new SignalingClient(url);
    clientRef.current = client;
    client.onStatus(setStatus);
    return () => {
      client.disconnect();
    };
  }, [url]);

  const connect = async () => {
    if (clientRef.current) {
      await clientRef.current.connect();
    }
  };

  const disconnect = () => {
    clientRef.current?.disconnect();
    setStatus('disconnected');
  };

  return (
    <SignalingContext.Provider value={{ client: clientRef.current, status, connect, disconnect }}>
      {children}
    </SignalingContext.Provider>
  );
}

export function useSignaling() {
  return useContext(SignalingContext);
}
