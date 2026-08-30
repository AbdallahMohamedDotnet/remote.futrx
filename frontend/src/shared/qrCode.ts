import { qrcodegen } from "./qrCodeEncoder.js";

export interface QrCodeMatrix {
  readonly size: number;
  isDark(x: number, y: number): boolean;
}

const QUIET_ZONE_MODULES = 1;

export class QRGenerator {
  /** Encodes text as a standards-compliant QR Code at medium error correction. */
  createMatrix(value: string): QrCodeMatrix {
    const segments = qrcodegen.QrSegment.makeSegments(value);
    const code = qrcodegen.QrCode.encodeSegments(
      segments,
      qrcodegen.QrCode.Ecc.MEDIUM,
      1,
      40,
      -1,
      false,
    );

    return {
      size: code.size,
      isDark: (x, y) => code.getModule(x, y),
    };
  }

  /** Renders QR text to the PNG data URL consumed by the enrollment image. */
  async createDataUrl(value: string, width: number): Promise<string> {
    const code = this.createMatrix(value);
    const canvas = document.createElement("canvas");
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Canvas 2D rendering is unavailable");

    canvas.width = width;
    canvas.height = width;

    const pixels = context.createImageData(width, width);
    const moduleCount = code.size + QUIET_ZONE_MODULES * 2;
    for (let y = 0; y < width; y++) {
      const moduleY = Math.floor(y * moduleCount / width) - QUIET_ZONE_MODULES;
      for (let x = 0; x < width; x++) {
        const moduleX = Math.floor(x * moduleCount / width) - QUIET_ZONE_MODULES;
        const color = code.isDark(moduleX, moduleY) ? 0 : 255;
        const offset = (y * width + x) * 4;
        pixels.data[offset] = color;
        pixels.data[offset + 1] = color;
        pixels.data[offset + 2] = color;
        pixels.data[offset + 3] = 255;
      }
    }

    context.putImageData(pixels, 0, 0);
    return canvas.toDataURL("image/png");
  }
}
