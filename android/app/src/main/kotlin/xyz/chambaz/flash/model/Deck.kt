package xyz.chambaz.flash.model

import com.google.gson.annotations.SerializedName

data class DeckItem(
    @SerializedName("name") val name: String,
    @SerializedName("due") val due: Int,
    @SerializedName("total") val total: Int,
)
