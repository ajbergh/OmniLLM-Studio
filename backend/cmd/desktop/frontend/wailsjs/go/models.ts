export namespace main {
	
	export class NativeCaptureCapabilitiesResponse {
	    supported: boolean;
	    ffmpeg_available: boolean;
	    audio_devices: string[];
	    video_devices: string[];
	    system_audio_devices: string[];
	    reconnect_supported: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new NativeCaptureCapabilitiesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.ffmpeg_available = source["ffmpeg_available"];
	        this.audio_devices = source["audio_devices"];
	        this.video_devices = source["video_devices"];
	        this.system_audio_devices = source["system_audio_devices"];
	        this.reconnect_supported = source["reconnect_supported"];
	        this.reason = source["reason"];
	    }
	}
	export class NativeCaptureRequest {
	    project_id: string;
	    fps: number;
	    audio_device: string;
	    capture_cursor: boolean;
	    capture_keystrokes: boolean;
	    reconnect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NativeCaptureRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project_id = source["project_id"];
	        this.fps = source["fps"];
	        this.audio_device = source["audio_device"];
	        this.capture_cursor = source["capture_cursor"];
	        this.capture_keystrokes = source["capture_keystrokes"];
	        this.reconnect = source["reconnect"];
	    }
	}

}

