package xyz.chambaz.flash.model

import com.google.gson.annotations.SerializedName

data class CardState(
    @SerializedName("ID") val id: Long,
    @SerializedName("Concept") val concept: String,
    @SerializedName("Reference") val reference: String,
)

data class EvaluateResult(
    @SerializedName("correct") val correct: Boolean,
    @SerializedName("accuracy") val accuracy: Double,
    @SerializedName("keywords_score") val keywordsScore: Double,
    @SerializedName("reshow_in_session") val reshowInSession: Boolean,
    @SerializedName("next_due") val nextDue: String,
)
