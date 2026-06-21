#!/usr/bin/env python3
"""
Évaluation de flashcards.
Présence des mots-clés : fuzzy matching seul (déterministe, fiable).
Qualité du sens : LLM seul (accuracy).
"""

import json
import re
import time
import unicodedata

import requests
from rapidfuzz import fuzz

OLLAMA_URL = "https://ollama.chambaz.xyz/api/chat"
USERNAME = "paulchambaz"
PASSWORD = "TPDCS0RG9zI2TjyGFo0pABvvoyK6iDFb"

MODEL = "qwen3:4b-instruct-2507-q4_K_M"
THRESHOLD = 0.7

SYSTEM_PROMPT = (
    "Tu évalues le SENS de la réponse d'un étudiant à une flashcard, "
    "comparée à une réponse de référence. Réponds par 'accuracy', un "
    "score entre 0 et 1 : 1 = sens complet et exact, 0.5 = partiel ou "
    "imprécis, 0 = faux ou hors-sujet. Tolère les fautes d'orthographe, "
    "ignore le style et la longueur."
)

JSON_SCHEMA = {
    "type": "object",
    "properties": {"accuracy": {"type": "number", "minimum": 0, "maximum": 1}},
    "required": ["accuracy"],
}

FLASHCARDS = {
    "Dot product": (
        "Somme des produits des composantes correspondantes "
        "($a \\cdot b = \\sum a_i b_i$), liée à la **projection** d'un "
        "vecteur sur l'autre et au **cosinus** de l'angle."
    ),
    "Matrix multiplication": (
        "Combine deux matrices : chaque élément résulte d'une **somme "
        "pondérée** des lignes et colonnes correspondantes ; représente la "
        "**composition** de deux transformations linéaires."
    ),
    "Determinant": (
        "Scalaire associé à une matrice carrée, mesure le **facteur "
        "d'échelle** du volume sous transformation ; **nul** si la matrice "
        "n'est pas inversible."
    ),
    "Rank": (
        "Nombre maximal de **vecteurs lignes indépendants** d'une matrice ; "
        "égale la **dimension** de l'espace image de la transformation."
    ),
    "Span": (
        "**Ensemble de toutes les combinaisons linéaires** possibles d'un "
        "jeu de vecteurs ; forme le plus petit **sous-espace vectoriel** "
        "les contenant."
    ),
    "Null space": (
        "**Ensemble des vecteurs** $x$ tels que $Ax = 0$ ; mesure le degré "
        "de **non-injectivité** de la transformation linéaire."
    ),
    "Column space": (
        "**Ensemble des combinaisons linéaires** des colonnes d'une "
        "matrice ; correspond à l'**image** de la transformation linéaire "
        "associée."
    ),
    "Eigenvalues / Eigenvectors": (
        "Vecteur $v$ non nul tel que $Av = \\lambda v$ ; $\\lambda$ est le "
        "**facteur d'étirement**, $v$ la **direction invariante** sous la "
        "transformation."
    ),
}

TEST_ANSWERS = {
    "Dot product": (
        "C'est la somme des produits des composante, ça donne la "
        "projeciton d'un vecteur sur un autre et le cosinus de l'angle "
        "entre eux."
    ),
    "Matrix multiplication": (
        "On combine deux matrices en faisant des sommes pondérées des "
        "lignes et colonnes."
    ),
    "Determinant": "C'est juste un nombre, je sais pas trop à quoi ça sert.",
    "Rank": (
        "Le rang correspond au nombre de vecteurs lignes indépendants, et "
        "ça donne la dimension de l'image de la transformation."
    ),
    "Span": (
        "C'est tous les vecteurs qu'on peut obtenir en additionnant et en "
        "multipliant par des scalaires les vecteurs de départ."
    ),
    "Null space": (
        "L'ensemble des vecteur x tel que Ax egal zero, ça mesure la non "
        "injectivité de la transformation."
    ),
    "Column space": "Les colonnes.",
    "Eigenvalues / Eigenvectors": (
        "C'est un vecteur non nul v tel que Av = lambda v, où lambda est "
        "le facteur d'étirement et v est la direction invariante sous la "
        "transformation."
    ),
}

session = requests.Session()
session.auth = (USERNAME, PASSWORD)


def normalize(s: str) -> str:
    s = s.lower().strip()
    s = "".join(
        c for c in unicodedata.normalize("NFD", s) if unicodedata.category(c) != "Mn"
    )
    return re.sub(r"[^\w\s]", " ", s)


def extract_bold_keywords(md: str) -> list:
    return re.findall(r"\*\*(.+?)\*\*", md)


def strip_bold(md: str) -> str:
    return md.replace("**", "")


def fuzzy_keywords_score(reference_md: str, user_answer: str) -> float:
    """Min des scores de similarité (sous-chaîne floue) sur chaque mot-clé :
    chaque mot-clé est essentiel, le moins bien couvert fixe le score."""
    keywords = extract_bold_keywords(reference_md)
    if not keywords:
        return 1.0
    norm_answer = normalize(user_answer)
    scores = [fuzz.partial_ratio(normalize(kw), norm_answer) for kw in keywords]
    return min(scores) / 100.0


def call_llm_accuracy(concept: str, reference_plain: str, user_answer: str) -> dict:
    payload = {
        "model": MODEL,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {
                "role": "user",
                "content": (
                    f"Concept: {concept}\nRéférence: {reference_plain}\n"
                    f"Réponse étudiant: {user_answer}"
                ),
            },
        ],
        "format": JSON_SCHEMA,
        "options": {"num_ctx": 512, "temperature": 0.1},
        "stream": False,
    }
    start = time.perf_counter()
    resp = session.post(OLLAMA_URL, json=payload, timeout=10)
    elapsed = round(time.perf_counter() - start, 3)

    print(f"DEBUG status={resp.status_code}")
    print(f"DEBUG body={resp.text[:300]!r}")

    try:
        data = json.loads(resp.json()["message"]["content"])
        accuracy = max(0.0, min(1.0, float(data["accuracy"])))
    except Exception as e:
        print(f"DEBUG erreur parsing: {type(e).__name__}: {e}")
        accuracy = 0.0

    return {"accuracy": accuracy, "latency_s": elapsed}


def evaluate(concept: str, reference_md: str, user_answer: str) -> dict:
    keywords_score = fuzzy_keywords_score(reference_md, user_answer)
    llm = call_llm_accuracy(concept, strip_bold(reference_md), user_answer)
    correct = keywords_score >= THRESHOLD and llm["accuracy"] >= THRESHOLD

    return {
        "keywords_score": keywords_score,
        "accuracy": llm["accuracy"],
        "correct": correct,
        "latency_s": llm["latency_s"],
    }


def main():
    for concept, reference_md in FLASHCARDS.items():
        user_answer = TEST_ANSWERS[concept]
        r = evaluate(concept, reference_md, user_answer)
        print("=" * 70)
        print(f"CONCEPT     : {concept}")
        print(f"RÉFÉRENCE   : {reference_md}")
        print(f"RÉPONSE USER: {user_answer}")
        print(
            f"SCORES      : keywords={r['keywords_score']:.2f}  "
            f"accuracy={r['accuracy']:.2f}"
        )
        print(f"FINAL       : correct={r['correct']}  latence={r['latency_s']}s")
        print()


if __name__ == "__main__":
    main()
