import { fromBinary, toBinary } from '@bufbuild/protobuf';
import {
  AnnotationPacketSchema,
  type AnnotationPacket,
} from '@dislackso/contracts/gen/dislackso/annotation/v1/annotation_pb';

/** The LiveKit data topic drawing packets travel on. */
export const ANNOTATION_TOPIC = 'annot';

export function encodePacket(p: AnnotationPacket): Uint8Array {
  return toBinary(AnnotationPacketSchema, p);
}

/** Undefined for bytes that are not a packet: they come from other clients. */
export function decodePacket(bytes: Uint8Array): AnnotationPacket | undefined {
  try {
    return fromBinary(AnnotationPacketSchema, bytes);
  } catch {
    return undefined;
  }
}
