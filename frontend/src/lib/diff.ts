export interface LineDiffPart {
  value: string;
  added?: true;
  removed?: true;
}

export function diffLines(oldText: string, newText: string): LineDiffPart[] {
  const oldLines = splitLines(oldText);
  const newLines = splitLines(newText);
  const oldLength = oldLines.length;
  const newLength = newLines.length;

  if (oldLength === 0) return newLines.map((value) => ({ value, added: true }));
  if (newLength === 0) return oldLines.map((value) => ({ value, removed: true }));

  const max = oldLength + newLength;
  const offset = max;
  let furthest = new Int32Array(max * 2 + 3);
  const trace: Int32Array[] = [];
  furthest[offset + 1] = 0;

  for (let distance = 0; distance <= max; distance++) {
    trace.push(furthest.slice());
    for (let diagonal = -distance; diagonal <= distance; diagonal += 2) {
      const index = offset + diagonal;
      let oldIndex =
        diagonal === -distance ||
        (diagonal !== distance && furthest[index - 1] < furthest[index + 1])
          ? furthest[index + 1]
          : furthest[index - 1] + 1;
      let newIndex = oldIndex - diagonal;

      while (
        oldIndex < oldLength &&
        newIndex < newLength &&
        oldLines[oldIndex] === newLines[newIndex]
      ) {
        oldIndex++;
        newIndex++;
      }

      furthest[index] = oldIndex;
      if (oldIndex >= oldLength && newIndex >= newLength) {
        return compactParts(backtrack(trace, oldLines, newLines, distance, offset));
      }
    }
  }

  return [];
}

function splitLines(text: string): string[] {
  return text.match(/[^\n]*\n|[^\n]+/g) ?? [];
}

function backtrack(
  trace: Int32Array[],
  oldLines: string[],
  newLines: string[],
  distance: number,
  offset: number
): LineDiffPart[] {
  const parts: LineDiffPart[] = [];
  let oldIndex = oldLines.length;
  let newIndex = newLines.length;

  for (let step = distance; step >= 0; step--) {
    const furthest = trace[step];
    const diagonal = oldIndex - newIndex;
    const previousDiagonal =
      diagonal === -step ||
      (diagonal !== step && furthest[offset + diagonal - 1] < furthest[offset + diagonal + 1])
        ? diagonal + 1
        : diagonal - 1;
    const previousOldIndex = furthest[offset + previousDiagonal];
    const previousNewIndex = previousOldIndex - previousDiagonal;

    while (oldIndex > previousOldIndex && newIndex > previousNewIndex) {
      oldIndex--;
      newIndex--;
      parts.push({ value: oldLines[oldIndex] });
    }

    if (step === 0) break;

    if (oldIndex === previousOldIndex) {
      newIndex--;
      parts.push({ value: newLines[newIndex], added: true });
    } else {
      oldIndex--;
      parts.push({ value: oldLines[oldIndex], removed: true });
    }
  }

  return parts.reverse();
}

function compactParts(parts: LineDiffPart[]): LineDiffPart[] {
  return parts.reduce<LineDiffPart[]>((merged, part) => {
    const previous = merged[merged.length - 1];
    if (previous && previous.added === part.added && previous.removed === part.removed) {
      previous.value += part.value;
    } else {
      merged.push({ ...part });
    }
    return merged;
  }, []);
}
