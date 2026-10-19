/** Someone in voice, dragged by a moderator onto another voice channel. */
export interface DraggedMember {
  readonly guildId: string;
  readonly userId: string;
}

const TYPE = 'application/x-dislackso-voice-member';

export function startMemberDrag(data: DataTransfer, member: DraggedMember): void {
  data.setData(TYPE, JSON.stringify(member));
  data.effectAllowed = 'move';
}

/** Whether a drag carries a member; while dragging, only the types are readable. */
export function carriesMember(data: DataTransfer): boolean {
  return data.types.includes(TYPE);
}

/** The member a drop carries, if it is a well-formed one. */
export function droppedMember(data: DataTransfer): DraggedMember | undefined {
  try {
    const v: unknown = JSON.parse(data.getData(TYPE));
    if (typeof v !== 'object' || v === null) return undefined;
    const { guildId, userId } = v as Record<string, unknown>;
    return typeof guildId === 'string' && typeof userId === 'string'
      ? { guildId, userId }
      : undefined;
  } catch {
    return undefined;
  }
}
