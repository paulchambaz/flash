package xyz.chambaz.flash.api

import com.google.gson.Gson
import com.google.gson.reflect.TypeToken
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import xyz.chambaz.flash.model.CardState
import xyz.chambaz.flash.model.DayActivity
import xyz.chambaz.flash.model.DeckItem
import xyz.chambaz.flash.model.EvaluateResult
import java.io.IOException
import java.util.concurrent.TimeUnit

class FlashApi(private val baseUrl: String, private val token: String) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .build()
    private val gson = Gson()
    private val json = "application/json; charset=utf-8".toMediaType()

    private fun get(path: String): String {
        val req = Request.Builder()
            .url("$baseUrl$path")
            .header("Authorization", "Bearer $token")
            .build()
        val resp = client.newCall(req).execute()
        val body = resp.body?.string() ?: ""
        if (!resp.isSuccessful) throw IOException("HTTP ${resp.code}: $body")
        return body
    }

    private fun post(path: String, payload: Any): String {
        val body = gson.toJson(payload).toRequestBody(json)
        val req = Request.Builder()
            .url("$baseUrl$path")
            .header("Authorization", "Bearer $token")
            .post(body)
            .build()
        val resp = client.newCall(req).execute()
        val respBody = resp.body?.string() ?: ""
        if (!resp.isSuccessful) throw IOException("HTTP ${resp.code}: $respBody")
        return respBody
    }

    fun listDecks(): List<DeckItem> {
        val body = get("/decks")
        val type = object : TypeToken<List<DeckItem>>() {}.type
        return gson.fromJson(body, type) ?: emptyList()
    }

    fun dueCards(deck: String): List<CardState> {
        val body = get("/decks/$deck/cards/due")
        val type = object : TypeToken<List<CardState>>() {}.type
        return gson.fromJson(body, type) ?: emptyList()
    }

    fun deckTotal(deck: String): Int {
        val body = get("/decks/$deck/total")
        val obj = gson.fromJson(body, Map::class.java)
        return (obj["total"] as? Double)?.toInt() ?: 0
    }

    fun activity(): List<DayActivity> {
        val body = get("/activity")
        val type = object : TypeToken<List<DayActivity>>() {}.type
        return gson.fromJson(body, type) ?: emptyList()
    }

    fun evaluate(deck: String, cardId: Long, answer: String, stepMs: Long, threshold: Float): EvaluateResult {
        val body = post("/decks/$deck/cards/evaluate", mapOf(
            "card_id" to cardId,
            "answer" to answer,
            "pace_seconds" to stepMs / 1000.0,
            "threshold" to threshold,
        ))
        return gson.fromJson(body, EvaluateResult::class.java)
    }
}
