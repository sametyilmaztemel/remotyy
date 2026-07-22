export class WebRTCClient {
  private pc: RTCPeerConnection | null = null;
  private dataChannels: Map<string, RTCDataChannel> = new Map();
  private config: RTCConfiguration;
  private onDataChannel: ((label: string, dc: RTCDataChannel) => void) | null = null;
  private onIceCandidate: ((candidate: RTCIceCandidateInit) => void) | null = null;
  private onStateChange: ((state: string) => void) | null = null;

  constructor(iceServers: string[] = ['stun:stun.l.google.com:19302']) {
    this.config = {
      iceServers: iceServers.map(url => ({ urls: url })),
      iceTransportPolicy: 'all',
      bundlePolicy: 'max-bundle',
      rtcpMuxPolicy: 'require',
    };
  }

  async createOffer(): Promise<RTCSessionDescriptionInit> {
    this.pc = this.createPeerConnection();
    const offer = await this.pc.createOffer({
      offerToReceiveAudio: false,
      offerToReceiveVideo: true,
    });
    await this.pc.setLocalDescription(offer);
    return offer;
  }

  async handleOffer(offer: RTCSessionDescriptionInit): Promise<RTCSessionDescriptionInit> {
    this.pc = this.createPeerConnection();
    await this.pc.setRemoteDescription(new RTCSessionDescription(offer));
    const answer = await this.pc.createAnswer();
    await this.pc.setLocalDescription(answer);
    return answer;
  }

  async handleAnswer(answer: RTCSessionDescriptionInit): Promise<void> {
    if (!this.pc) return;
    await this.pc.setRemoteDescription(new RTCSessionDescription(answer));
  }

  async addIceCandidate(candidate: RTCIceCandidateInit): Promise<void> {
    try {
      await this.pc?.addIceCandidate(new RTCIceCandidate(candidate));
    } catch (e) {
      console.warn('ICE candidate error:', e);
    }
  }

  createDataChannel(label: string): RTCDataChannel | null {
    if (!this.pc) return null;
    const dc = this.pc.createDataChannel(label, {
      ordered: true,
    });
    this.dataChannels.set(label, dc);
    this.onDataChannel?.(label, dc);
    return dc;
  }

  createVideoTrack(): MediaStreamTrack | null {
    // In browser, we receive video from the host
    // The host sends screen capture as a video track
    return null;
  }

  close(): void {
    this.dataChannels.forEach(dc => dc.close());
    this.dataChannels.clear();
    this.pc?.close();
    this.pc = null;
  }

  get connectionState(): string {
    return this.pc?.connectionState || 'new';
  }

  onDataChannelCb(fn: (label: string, dc: RTCDataChannel) => void): void {
    this.onDataChannel = fn;
  }

  onIceCandidateCb(fn: (candidate: RTCIceCandidateInit) => void): void {
    this.onIceCandidate = fn;
  }

  onStateChangeCb(fn: (state: string) => void): void {
    this.onStateChange = fn;
  }

  private createPeerConnection(): RTCPeerConnection {
    const pc = new RTCPeerConnection(this.config);

    pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.onIceCandidate?.(event.candidate.toJSON());
      }
    };

    pc.ondatachannel = (event) => {
      const dc = event.channel;
      this.dataChannels.set(dc.label, dc);
      this.onDataChannel?.(dc.label, dc);
    };

    pc.onconnectionstatechange = () => {
      this.onStateChange?.(pc.connectionState);
    };

    pc.oniceconnectionstatechange = () => {
      this.onStateChange?.(pc.iceConnectionState);
    };

    return pc;
  }
}
