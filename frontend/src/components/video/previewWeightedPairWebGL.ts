import type { CanonicalTransitionPairPixelComposition } from '../../video/renderContractTransitionPairPixels';
import { resolveWeightedTransitionPairKernelWeights } from '../../video/renderContractTransitionPairPixelKernel';

const VERTEX_SHADER_SOURCE = `#version 300 es
in vec2 a_position;
out vec2 v_uv;

void main() {
  v_uv = (a_position + 1.0) * 0.5;
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const FRAGMENT_SHADER_SOURCE = `#version 300 es
precision highp float;

uniform sampler2D u_outgoing;
uniform sampler2D u_incoming;
uniform float u_outgoing_weight;
uniform float u_incoming_weight;
uniform float u_black_weight;

in vec2 v_uv;
out vec4 out_color;

float srgb_to_linear_channel(float value) {
  return value <= 0.04045
    ? value / 12.92
    : pow((value + 0.055) / 1.055, 2.4);
}

vec3 srgb_to_linear(vec3 value) {
  return vec3(
    srgb_to_linear_channel(value.r),
    srgb_to_linear_channel(value.g),
    srgb_to_linear_channel(value.b)
  );
}

float linear_to_srgb_channel(float value) {
  value = clamp(value, 0.0, 1.0);
  return value <= 0.0031308
    ? 12.92 * value
    : (1.055 * pow(value, 1.0 / 2.4)) - 0.055;
}

vec3 linear_to_srgb(vec3 value) {
  return vec3(
    linear_to_srgb_channel(value.r),
    linear_to_srgb_channel(value.g),
    linear_to_srgb_channel(value.b)
  );
}

void main() {
  vec4 outgoing = texture(u_outgoing, v_uv);
  vec4 incoming = texture(u_incoming, v_uv);
  float output_alpha = clamp(
    (u_outgoing_weight * outgoing.a)
      + (u_incoming_weight * incoming.a)
      + u_black_weight,
    0.0,
    1.0
  );
  if (output_alpha <= 0.0) {
    out_color = vec4(0.0);
    return;
  }

  vec3 premultiplied_linear =
    (u_outgoing_weight * outgoing.a * srgb_to_linear(outgoing.rgb))
      + (u_incoming_weight * incoming.a * srgb_to_linear(incoming.rgb));
  vec3 straight_linear = clamp(premultiplied_linear / output_alpha, 0.0, 1.0);
  out_color = vec4(linear_to_srgb(straight_linear), output_alpha);
}
`;

export interface PreviewWeightedPairWebGLCompositor {
  render(
    outgoing: HTMLCanvasElement,
    incoming: HTMLCanvasElement,
    composition: CanonicalTransitionPairPixelComposition,
  ): void;
  dispose(): void;
}

/**
 * Create the real-time weighted-pair compositor used only by normal playback.
 *
 * Geometry remains rasterized by the same canonical 2D layer painter used by
 * deterministic preview. WebGL2 owns only the expensive full-frame pixel
 * composition: straight-alpha sRGB inputs are decoded to linear sRGB,
 * accumulated in premultiplied form with the canonical pair weights, converted
 * back to straight alpha, and encoded to sRGB. This is the GPU equivalent of
 * composeWeightedTransitionPairRgba, while the byte-exact CPU kernel remains
 * the deterministic/static reference path.
 */
export function createPreviewWeightedPairWebGLCompositor(
  canvas: HTMLCanvasElement,
): PreviewWeightedPairWebGLCompositor | null {
  const maybeGL = canvas.getContext('webgl2', {
    alpha: true,
    antialias: false,
    depth: false,
    stencil: false,
    premultipliedAlpha: false,
    preserveDrawingBuffer: true,
  });
  if (!maybeGL) return null;
  const gl: WebGL2RenderingContext = maybeGL;

  const vertexShader = compileShader(gl, gl.VERTEX_SHADER, VERTEX_SHADER_SOURCE);
  const fragmentShader = compileShader(gl, gl.FRAGMENT_SHADER, FRAGMENT_SHADER_SOURCE);
  const program = gl.createProgram();
  if (!program) {
    gl.deleteShader(vertexShader);
    gl.deleteShader(fragmentShader);
    throw new Error('weighted playback WebGL2 could not create shader program');
  }
  gl.attachShader(program, vertexShader);
  gl.attachShader(program, fragmentShader);
  gl.linkProgram(program);
  gl.deleteShader(vertexShader);
  gl.deleteShader(fragmentShader);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const reason = gl.getProgramInfoLog(program) || 'unknown program link error';
    gl.deleteProgram(program);
    throw new Error(`weighted playback WebGL2 program link failed: ${reason}`);
  }

  const vao = gl.createVertexArray();
  const buffer = gl.createBuffer();
  const outgoingTexture = createTexture(gl);
  const incomingTexture = createTexture(gl);
  if (!vao || !buffer) {
    if (vao) gl.deleteVertexArray(vao);
    if (buffer) gl.deleteBuffer(buffer);
    gl.deleteTexture(outgoingTexture);
    gl.deleteTexture(incomingTexture);
    gl.deleteProgram(program);
    throw new Error('weighted playback WebGL2 could not create fullscreen geometry');
  }

  gl.bindVertexArray(vao);
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(
    gl.ARRAY_BUFFER,
    new Float32Array([
      -1, -1,
      1, -1,
      -1, 1,
      -1, 1,
      1, -1,
      1, 1,
    ]),
    gl.STATIC_DRAW,
  );
  const positionLocation = gl.getAttribLocation(program, 'a_position');
  if (positionLocation < 0) {
    disposeResources();
    throw new Error('weighted playback WebGL2 program is missing a_position');
  }
  gl.enableVertexAttribArray(positionLocation);
  gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0);
  gl.bindVertexArray(null);

  const outgoingSampler = requireUniform(gl, program, 'u_outgoing');
  const incomingSampler = requireUniform(gl, program, 'u_incoming');
  const outgoingWeight = requireUniform(gl, program, 'u_outgoing_weight');
  const incomingWeight = requireUniform(gl, program, 'u_incoming_weight');
  const blackWeight = requireUniform(gl, program, 'u_black_weight');

  gl.useProgram(program);
  gl.uniform1i(outgoingSampler, 0);
  gl.uniform1i(incomingSampler, 1);
  gl.disable(gl.BLEND);
  gl.disable(gl.DEPTH_TEST);
  gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
  gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
  gl.pixelStorei(gl.UNPACK_COLORSPACE_CONVERSION_WEBGL, gl.NONE);

  let disposed = false;

  return {
    render(outgoing, incoming, composition) {
      if (disposed) throw new Error('weighted playback WebGL2 compositor is disposed');
      if (outgoing.width !== canvas.width || outgoing.height !== canvas.height
        || incoming.width !== canvas.width || incoming.height !== canvas.height) {
        throw new Error('weighted playback WebGL2 inputs must match the output canvas dimensions');
      }
      const weights = resolveWeightedTransitionPairKernelWeights(composition);
      gl.viewport(0, 0, canvas.width, canvas.height);
      gl.useProgram(program);
      gl.bindVertexArray(vao);
      uploadTexture(gl, outgoingTexture, 0, outgoing);
      uploadTexture(gl, incomingTexture, 1, incoming);
      gl.uniform1f(outgoingWeight, weights.outgoing);
      gl.uniform1f(incomingWeight, weights.incoming);
      gl.uniform1f(blackWeight, weights.black);
      gl.drawArrays(gl.TRIANGLES, 0, 6);
      gl.bindVertexArray(null);
      const error = gl.getError();
      if (error !== gl.NO_ERROR) {
        throw new Error(`weighted playback WebGL2 draw failed with GL error ${error}`);
      }
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      disposeResources();
    },
  };

  function disposeResources() {
    gl.deleteTexture(outgoingTexture);
    gl.deleteTexture(incomingTexture);
    gl.deleteBuffer(buffer);
    gl.deleteVertexArray(vao);
    gl.deleteProgram(program);
  }
}

function compileShader(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error('weighted playback WebGL2 could not create shader');
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const reason = gl.getShaderInfoLog(shader) || 'unknown shader compile error';
    gl.deleteShader(shader);
    throw new Error(`weighted playback WebGL2 shader compile failed: ${reason}`);
  }
  return shader;
}

function createTexture(gl: WebGL2RenderingContext): WebGLTexture {
  const texture = gl.createTexture();
  if (!texture) throw new Error('weighted playback WebGL2 could not create texture');
  gl.bindTexture(gl.TEXTURE_2D, texture);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  return texture;
}

function uploadTexture(
  gl: WebGL2RenderingContext,
  texture: WebGLTexture,
  unit: number,
  source: HTMLCanvasElement,
): void {
  gl.activeTexture(gl.TEXTURE0 + unit);
  gl.bindTexture(gl.TEXTURE_2D, texture);
  gl.texImage2D(
    gl.TEXTURE_2D,
    0,
    gl.RGBA,
    gl.RGBA,
    gl.UNSIGNED_BYTE,
    source,
  );
}

function requireUniform(
  gl: WebGL2RenderingContext,
  program: WebGLProgram,
  name: string,
): WebGLUniformLocation {
  const location = gl.getUniformLocation(program, name);
  if (!location) throw new Error(`weighted playback WebGL2 program is missing ${name}`);
  return location;
}
